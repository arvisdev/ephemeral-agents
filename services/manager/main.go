// Command manager is the M5 Manager Agent on vm-xa — the persistent,
// human-facing front-of-house component (M5 install doc §1).
//
// Per request from the gateway:
//  1. git clone --depth 1 the Available Agent List (read token).
//  2. Search index.json for the agent matching the request's capability.
//  3. Locate the folder: persona + skill-and-tools + behavioral spec.
//  4. Compose the M1 task message (schemas/message-v1.json) — the manager is
//     the ONLY writer of ttl_s: max(3 × B1 boot-time p50, brief estimate) —
//     and publish it to the agent.tasks topic exchange with routing key
//     agent.tasks.<capability> (D-routing).
//  5. Consume agent.tasks.response filtered by task_id → write the human
//     report (outcome + artifact pointer) to /reports/<task_id>.json.
//
// Unit tests (message_test.go) cover the pure parts: message composition +
// schema conformance, the ttl_s rule, index search, and response filtering.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// pending tracks tasks this manager published, awaiting the correlated
// response on agent.tasks.response.
type pending struct {
	mu      sync.Mutex
	waiting map[string]TaskRequest // task_id -> originating request
}

func newPending() *pending { return &pending{waiting: map[string]TaskRequest{}} }

func (p *pending) add(id string, req TaskRequest) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.waiting[id] = req
}

func (p *pending) take(id string) (TaskRequest, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	req, ok := p.waiting[id]
	delete(p.waiting, id)
	return req, ok
}

type server struct {
	cfg        config
	pending    *pending
	amqpCancel chan struct{}
}

type config struct {
	rabbitURL     string
	catalogRepo   string
	readToken     string
	tasksExchange string
	responseQueue string
	reportsDir    string
	bootP50S      int64
	briefEstS     int64
	repoTarget    string
	requester     string
	listen        string
}

func loadConfig() config {
	c := config{
		rabbitURL:     getenv(EnvRabbitMQURL, "amqp://guest:guest@localhost:5672/"),
		catalogRepo:   os.Getenv(EnvAgentListRepo),
		readToken:     os.Getenv(EnvGitHubReadToken),
		tasksExchange: getenv(EnvTasksExchange, "agent.tasks"),
		responseQueue: getenv(EnvResponseQueue, "agent.tasks.response"),
		reportsDir:    getenv(EnvReportsDir, "/reports"),
		repoTarget:    os.Getenv(EnvRepoTarget),
		requester:     getenv(EnvRequester, "desktop-engineer"),
		listen:        getenv(EnvListen, "0.0.0.0:8086"),
	}
	c.bootP50S = getInt64(EnvBootP50S, DefaultBootP50S)
	c.briefEstS = getInt64(EnvBriefEstimateS, DefaultBriefEstS)
	return c
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getInt64(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		var n int64
		if _, err := fmt.Sscan(v, &n); err == nil {
			return n
		}
		log.Printf("warn: bad %s %q — using default %d", k, v, def)
	}
	return def
}

func main() {
	cfg := loadConfig()
	if cfg.catalogRepo == "" {
		log.Fatalf("%s is required", EnvAgentListRepo)
	}
	if cfg.repoTarget == "" {
		log.Fatalf("%s is required", EnvRepoTarget)
	}
	if err := os.MkdirAll(cfg.reportsDir, 0o755); err != nil {
		log.Fatalf("reports dir %s: %v", cfg.reportsDir, err)
	}
	s := &server{cfg: cfg, pending: newPending(), amqpCancel: make(chan struct{})}

	conn, ch, err := s.connectAMQP()
	if err != nil {
		log.Fatalf("rabbitmq: %v", err)
	}
	defer conn.Close()
	defer ch.Close()
	// The response queue is declared by M0 §3 (no exchange binding); the
	// manager only consumes it — publishing responses is M2's job, on the
	// default exchange.
	if _, err := ch.QueueDeclarePassive(cfg.responseQueue, true, false, false, false, nil); err != nil {
		log.Printf("warn: passive declare of %s failed (%v) — declaring durable", cfg.responseQueue, err)
		if _, err := ch.QueueDeclare(cfg.responseQueue, true, false, false, false, nil); err != nil {
			log.Fatalf("declare %s: %v", cfg.responseQueue, err)
		}
	}
	go s.consumeResponses(ch)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("POST /tasks", s.handleTask)
	mux.HandleFunc("GET /reports/", s.handleGetReport)

	log.Printf("manager listening on %s (exchange=%s queue=%s reports=%s)",
		cfg.listen, cfg.tasksExchange, cfg.responseQueue, cfg.reportsDir)
	if err := http.ListenAndServe(cfg.listen, mux); err != nil {
		log.Fatal(err)
	}
}

// catalogCloneURL embeds the read token for an https clone (never printed).
func (c config) catalogCloneURL() string {
	repo := c.catalogRepo
	if c.readToken == "" {
		return repo
	}
	if strings.HasPrefix(repo, "git@github.com:") {
		repo = "https://github.com/" + strings.TrimPrefix(repo, "git@github.com:")
	}
	repo = strings.TrimSuffix(repo, ".git")
	return "https://x-access-token:" + c.readToken + "@github.com/" +
		strings.TrimPrefix(repo, "https://github.com/") + ".git"
}

// cloneCatalog runs step 1: git clone --depth 1 of the Available Agent List
// into a temp dir; returns the path (caller cleans up).
func (s *server) cloneCatalog() (string, error) {
	dir, err := os.MkdirTemp("", "agent-list-")
	if err != nil {
		return "", err
	}
	url := s.cfg.catalogCloneURL()
	cmd := exec.Command("git", "clone", "--depth", "1", "--quiet", url, dir)
	// Keep the token out of ps/logs: pass the URL via env expansion, not argv.
	cmd.Env = append(os.Environ(), "GIT_ASKPASS=/bin/true", "GIT_TERMINAL_PROMPT=0")
	cmd.Stderr = nil
	if out, err := cmd.CombinedOutput(); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("git clone of the agent catalog failed: %v: %s", err, redact(string(out), s.cfg.readToken))
	}
	return dir, nil
}

func redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}

// resolveAgent runs steps 2–3: read index.json, match capability, read the
// behavioral spec from the agent's folder (fall back to the index's spec_ref).
func resolveAgent(dir string, req TaskRequest) (AgentIndexEntry, string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return AgentIndexEntry{}, "", fmt.Errorf("reading index.json: %w", err)
	}
	idx, err := ParseAgentIndex(raw)
	if err != nil {
		return AgentIndexEntry{}, "", err
	}
	agent, err := MatchAgent(idx, req.AgentCapability)
	if err != nil {
		return AgentIndexEntry{}, "", err
	}
	spec := agent.SpecRef
	if b, err := os.ReadFile(filepath.Join(dir, agent.Folder, "behavioral-spec.md")); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		spec = string(b)
	}
	return agent, spec, nil
}

// handleTask is steps 1–4 of the manager entrypoint, per gateway request.
func (s *server) handleTask(w http.ResponseWriter, r *http.Request) {
	var req TaskRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.AgentCapability) == "" || strings.TrimSpace(req.TaskBrief) == "" {
		httpError(w, http.StatusBadRequest, "title, agent_capability and task_brief are required")
		return
	}

	dir, err := s.cloneCatalog()
	if err != nil {
		// M5 §5 F2 policy: catalog unreachable ⇒ fail fast, human-visible error.
		httpError(w, http.StatusBadGateway, err.Error())
		return
	}
	defer os.RemoveAll(dir)

	agent, spec, err := resolveAgent(dir, req)
	if err != nil {
		httpError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Brief estimate: an explicit ttl_s in the request is the engineer's
	// estimate; otherwise the configured default estimate. The manager still
	// owns the final value via the max() rule.
	est := s.cfg.briefEstS
	if req.TTLS > 0 {
		est = req.TTLS
	}
	agent.SpecRef = spec // the behavioral spec the manager passes through (content or spec_ref)
	msg, err := ComposeTaskMessage(req, agent, s.cfg.catalogRepo, s.cfg.repoTarget, s.cfg.requester, s.cfg.bootP50S, est)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "compose task message: "+err.Error())
		return
	}
	wire, err := msg.MarshalStrict()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "marshal task message: "+err.Error())
		return
	}
	if err := s.publish(wire, RoutingKey(req.AgentCapability)); err != nil {
		httpError(w, http.StatusBadGateway, "publish to "+s.cfg.tasksExchange+": "+err.Error())
		return
	}
	s.pending.add(msg.TaskID, req)
	log.Printf("published task_id=%s routing_key=%s ttl_s=%d agent_ref=%s",
		msg.TaskID, RoutingKey(req.AgentCapability), *msg.TTLS, msg.AgentRef)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]any{
		"task_id": msg.TaskID, "agent_ref": msg.AgentRef, "ttl_s": *msg.TTLS,
	})
}

func (s *server) publish(body []byte, routingKey string) error {
	conn, ch, err := s.connectAMQP()
	if err != nil {
		return err
	}
	defer conn.Close()
	defer ch.Close()
	if err := ch.ExchangeDeclarePassive(s.cfg.tasksExchange, "topic", true, false, false, false, nil); err != nil {
		log.Printf("passive exchange declare failed (%v) — declaring topic exchange", err)
		if err := ch.ExchangeDeclare(s.cfg.tasksExchange, "topic", true, false, false, false, nil); err != nil {
			return err
		}
	}
	return ch.PublishWithContext(context.Background(), s.cfg.tasksExchange, routingKey, false, false,
		amqp.Publishing{ContentType: "application/json", Body: body, DeliveryMode: amqp.Persistent})
}

func (s *server) connectAMQP() (*amqp.Connection, *amqp.Channel, error) {
	conn, err := amqp.Dial(s.cfg.rabbitURL)
	if err != nil {
		return nil, nil, err
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return conn, ch, nil
}

// consumeResponses is step 5: consume agent.tasks.response, filter by the
// task_ids this manager published, write /reports/<task_id>.json.
func (s *server) consumeResponses(ch *amqp.Channel) {
	deliveries, err := ch.Consume(s.cfg.responseQueue, "", true, false, false, false, nil)
	if err != nil {
		log.Printf("consume %s: %v", s.cfg.responseQueue, err)
		return
	}
	for {
		select {
		case <-s.amqpCancel:
			return
		case d, ok := <-deliveries:
			if !ok {
				log.Printf("response delivery channel closed")
				return
			}
			var resp TaskResponse
			if err := json.Unmarshal(d.Body, &resp); err != nil {
				log.Printf("response not JSON — ignored: %v", err)
				continue
			}
			req, ok := s.pending.take(resp.TaskID)
			if !ok || !ResponseMatches(resp, resp.TaskID) {
				continue // not this task — other managers may share the queue
			}
			reason := ""
			if b := d.Headers["reason"]; b != nil {
				if str, ok := b.(string); ok {
					reason = str
				}
			}
			report, err := BuildReport(resp, reason)
			if err != nil {
				log.Printf("build report: %v", err)
				continue
			}
			path := filepath.Join(s.cfg.reportsDir, resp.TaskID+".json")
			b, _ := json.MarshalIndent(report, "", "  ")
			if err := writeAtomic(path, b); err != nil {
				log.Printf("write %s: %v", path, err)
				s.pending.add(resp.TaskID, req) // retry on the next delivery/re-poll
				continue
			}
			log.Printf("report written: %s (status=%s artifact=%s)", path, report.Status, report.Artifact)
		}
	}
}

func writeAtomic(path string, b []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// handleGetReport serves the persisted report for the gateway to proxy.
func (s *server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/reports/")
	if !uuidOK(id) {
		httpError(w, http.StatusBadRequest, "malformed task_id")
		return
	}
	b, err := os.ReadFile(filepath.Join(s.cfg.reportsDir, id+".json"))
	if err != nil {
		httpError(w, http.StatusNotFound, "no report yet for "+id)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}

func uuidOK(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
	}
	return true
}

func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
