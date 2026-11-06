// Package lifecycle implements the M2 lifecycle service: the compose-network
// HTTP API (spawn/status/reap/health/report) plus the 30 s label-based reaper
// loop, all backed by the Docker CLI over the mounted socket.
package lifecycle

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/dockercli"
	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// Service drives Docker and serves the lifecycle API.
type Service struct {
	cli          *dockercli.CLI
	log          *slog.Logger
	image        string        // pinned POC_IMAGE — never pulled at activity time
	defaultTTL   time.Duration // POC_TTL_DEFAULT
	tasksDir     string        // /var/poc/tasks for per-task logs
	reapInterval time.Duration // REAPER_INTERVAL
	reapGrace    time.Duration // REAPER_GRACE — wait past poc.expires before reaping
	m3Env        map[string]string // M3 §2 per-run env (secrets from compose env_file)
	cpuLimit     float64
	memLimit     int64

	mu          sync.Mutex
	lastReapLog []string
	reapCount   int
}

// New builds the service; the CLI runs docker commands against the socket.
func New(cli *dockercli.CLI, log *slog.Logger, image string, defaultTTL, reapInterval, reapGrace time.Duration, tasksDir string, cpuLimit float64, memLimit int64) *Service {
	return &Service{
		cli: cli, log: log, image: image, defaultTTL: defaultTTL,
		m3Env: m3FromEnv(),
		tasksDir: tasksDir, reapInterval: reapInterval, reapGrace: reapGrace, cpuLimit: cpuLimit, memLimit: memLimit,
	}
}

// ImagePresent implements the no-pull deploy gate: the pinned image must
// exist locally or the service refuses to serve (M2 §2).
func (s *Service) ImagePresent(ctx context.Context) error {
	return s.cli.ImagePresent(ctx, s.image)
}

// Spawn creates and starts the ephemeral executor: pinned image, config env,
// per-task work volume, owner/task/expiry labels, resource limits. It never
// pulls the image — an absent image is a spawn refusal, by design.
func (s *Service) Spawn(ctx context.Context, cfg taskconfig.TaskConfig) (taskconfig.SpawnResponse, error) {
	if err := cfg.Validate(); err != nil {
		return taskconfig.SpawnResponse{}, err
	}
	if err := s.cli.ImagePresent(ctx, s.image); err != nil {
		return taskconfig.SpawnResponse{}, fmt.Errorf("pinned image %q absent locally (no-pull rule): %w", s.image, err)
	}

	expires := cfg.ExpiryTime(s.defaultTTL, time.Now())
	workDir := filepath.Join(s.tasksDir, cfg.TaskID)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return taskconfig.SpawnResponse{}, fmt.Errorf("work dir: %w", err)
	}

	env := []string{
		"TASK_ID=" + cfg.TaskID,
		"AGENT_REF=" + cfg.AgentRef,
		"BEHAVIORAL_SPEC=" + cfg.BehavioralSpec,
		"REPO_TARGET=" + cfg.RepoTarget,
		"REQUESTER=" + cfg.Requester,
		"TTL=" + strconv.FormatInt(int64(cfg.EffectiveTTL(s.defaultTTL)/time.Second), 10),
		"INJECT=" + cfg.Inject,
	}

	// M3 (§2/§2b): per-executor env injection — secrets live in the lifecycle
	// process env (compose env_file from /opt/poc/secrets/*), never in images.
	passThrough := map[string]string{
		"AGENT_REPO_URL":      cfg.AgentRepoURL,
		"REPO_FOLDER":         cfg.RepoFolder,
		"HINDSIGHT_URL":       s.m3Env["HINDSIGHT_URL"],
		"LLM_WIKI_REPO_URL":   s.m3Env["LLM_WIKI_REPO_URL"],
		"LLM_WIKI_READ_TOKEN": s.m3Env["LLM_WIKI_READ_TOKEN"],
		"MODEL_API_KEY":       s.m3Env["MODEL_API_KEY"],
		"ANTHROPIC_API_KEY":   s.m3Env["ANTHROPIC_API_KEY"],
		"TASK_PROMPT":         cfg.TaskPrompt,
		"MAX_TURNS":           cfg.MaxTurnsLabel(),
		"MODEL":               cfg.Model,
	}
	switch cfg.Executor {
	case "hermes":
		env = append(env, "GITHUB_READ_TOKEN="+s.m3Env["GITHUB_READ_TOKEN"])
		for k, v := range passThrough {
			if v != "" {
				env = append(env, k+"="+v)
			}
		}
	case "coder":
		// GITHUB_TOKEN: the scoped App installation token, issued at spawn (§1c).
		tok, err := s.issueToken()
		if err != nil {
			return taskconfig.SpawnResponse{}, fmt.Errorf("issue token (§1c): %w", err)
		}
		env = append(env, "GITHUB_TOKEN="+tok, "ANTHROPIC_API_KEY="+s.m3Env["ANTHROPIC_API_KEY"])
		for k, v := range passThrough {
			if v != "" {
				env = append(env, k+"="+v)
			}
		}
	}
	cid, err := s.cli.Run(ctx, dockercli.RunSpec{
		TaskID: cfg.TaskID, Image: s.imageFor(cfg.Executor), Env: env, WorkDir: workDir,
		Expires: expires, CPUs: s.cpuLimit, MemBytes: s.memLimit,
	})
	if err != nil {
		return taskconfig.SpawnResponse{}, fmt.Errorf("container create: %w", err)
	}
	s.log.Info("spawn", "event", "spawn", "task_id", cfg.TaskID, "container_id", cid)
	return taskconfig.SpawnResponse{ContainerID: cid, Expires: expires.UTC().Format(time.RFC3339)}, nil
}

// Status resolves a task's container state ("gone" when absent — a reaped
// container is a normal poll result, not an error).
func (s *Service) Status(ctx context.Context, taskID string) (taskconfig.StatusResponse, error) {
	st, err := s.cli.Inspect(ctx, taskID)
	if err == dockercli.ErrNotFound {
		return taskconfig.StatusResponse{TaskID: taskID, State: "gone"}, nil
	}
	if err != nil {
		return taskconfig.StatusResponse{}, err
	}
	return taskconfig.StatusResponse{
		TaskID: taskID, ContainerID: st.ContainerID, State: st.Status,
		ExitCode: st.ExitCode, Expires: st.Expires,
	}, nil
}

// Reap is idempotent: capture logs to the per-task file, docker rm -f, remove
// the per-task work dir; an absent task is a no-op success.
func (s *Service) Reap(ctx context.Context, taskID, reason string) error {
	if st, err := s.cli.Inspect(ctx, taskID); err == nil {
		if err := s.cli.Logs(ctx, st.ContainerID, filepath.Join(s.tasksDir, taskID+".log")); err != nil {
			s.log.Warn("log capture failed", "task_id", taskID, "err", err)
		}
	}
	if err := s.cli.Remove(ctx, taskID); err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	// The per-task work dir is the "per-task volume" at POC scale; remove last.
	_ = os.RemoveAll(filepath.Join(s.tasksDir, taskID))
	s.mu.Lock()
	s.reapCount++
	s.lastReapLog = append([]string{fmt.Sprintf("%s task_id=%s reason=%s",
		time.Now().UTC().Format(time.RFC3339), taskID, reason)}, s.lastReapLog...)
	if len(s.lastReapLog) > 10 {
		s.lastReapLog = s.lastReapLog[:10]
	}
	s.mu.Unlock()
	s.log.Info("reap", "event", "reap", "task_id", taskID, "reason", reason)
	return nil
}

// RunReaper is the backstop loop (REAPER_INTERVAL): reap any POC-owned
// container past poc.expires, or with a missing/unparsable expiry label.
// Temporal reachability is deliberately not consulted — labels only.
func (s *Service) RunReaper(ctx context.Context) {
	t := time.NewTicker(s.reapInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.reapSweep(ctx)
		}
	}
}

func (s *Service) reapSweep(ctx context.Context) {
	// reaper grace: a container is not reaped until it is past poc.expires
	// by REAPER_GRACE — a live worker's activity polls every 2s and observes
	// the container's own exit (complete/failed) well before the reaper may
	// remove it; the grace exists so the poll can never lose that race.
	ids, tasks, reasons, err := s.cli.ExpiredList(ctx, time.Now().Add(-s.reapGrace))
	if err != nil {
		s.log.Warn("reaper list failed", "err", err)
		return
	}
	for i, id := range ids {
		taskID := tasks[i]
		if taskID == "" {
			taskID = id[:12]
		}
		if err := s.Reap(ctx, taskID, "reaper:"+reasons[i]); err != nil {
			s.log.Warn("reaper reap failed", "task_id", taskID, "err", err)
		}
	}
}

// ---- HTTP handlers (compose-network only; LISTEN=0.0.0.0:8090) ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// Handler mounts the API routes.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /spawn", s.handleSpawn)
	mux.HandleFunc("GET /status", s.handleStatus)
	mux.HandleFunc("POST /reap", s.handleReap)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /report", s.handleReport)
	return mux
}

func (s *Service) handleSpawn(w http.ResponseWriter, r *http.Request) {
	var cfg taskconfig.TaskConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json: " + err.Error()})
		return
	}
	resp, err := s.Spawn(r.Context(), cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Service) handleStatus(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task")
	if taskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task query param required"})
		return
	}
	st, err := s.Status(r.Context(), taskID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (s *Service) handleReap(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("task")
	if taskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task query param required"})
		return
	}
	reason := r.URL.Query().Get("reason")
	if reason == "" {
		reason = "api"
	}
	// Idempotent: reaping an absent task succeeds silently.
	if err := s.Reap(r.Context(), taskID, reason); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"task_id": taskID, "reaped": "true"})
}

func (s *Service) handleReport(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"reap_count":    s.reapCount,
		"last_reaps":    s.lastReapLog,
		"pinned_image":  s.image,
		"ttl_default_s": int(s.defaultTTL / time.Second),
	})
}

// imageFor resolves the pinned image per executor type (M3 §2': POC_IMAGE is
// a config switch). Empty type → the M2 stub pin (rollback rule).
func (s *Service) imageFor(executor string) string {
	switch executor {
	case "hermes":
		if v := s.m3Env["POC_IMAGE_HERMES"]; v != "" {
			return v
		}
	case "coder":
		if v := s.m3Env["POC_IMAGE_CODER"]; v != "" {
			return v
		}
	}
	return s.image
}

// issueToken shells out to the M3 token issuance script (§1c): JWT-signed
// App token, TTL ≤ 1 h. Skipped (error) when the App is not configured —
// coder tasks cannot run without it.
func (s *Service) issueToken() (string, error) {
	if s.m3Env["GITHUB_APP_ID"] == "" || s.m3Env["INSTALLATION_ID"] == "" {
		return "", fmt.Errorf("GitHub App not configured (GITHUB_APP_ID/INSTALLATION_ID absent) — §1c required for coder tasks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/opt/poc/deploy/m3/issue-token.sh").Output()
	if err != nil {
		return "", fmt.Errorf("issue-token.sh: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
// m3FromEnv collects the M3 §2 per-run env keys the lifecycle process
// carries (compose env_file from /opt/poc/secrets/*). Missing keys map to
// "" — coder/hermes features that need them fail at spawn with a clear error.
func m3FromEnv() map[string]string {
	keys := []string{
		"HINDSIGHT_URL", "LLM_WIKI_REPO_URL", "LLM_WIKI_READ_TOKEN",
		"MODEL_API_KEY", "ANTHROPIC_API_KEY", "GITHUB_READ_TOKEN",
		"GITHUB_APP_ID", "INSTALLATION_ID", "POC_IMAGE_HERMES", "POC_IMAGE_CODER",
	}
	out := map[string]string{}
	for _, k := range keys {
		out[k] = os.Getenv(k)
	}
	return out
}