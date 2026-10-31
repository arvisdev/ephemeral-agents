package main

// Pure message-composition and response-filtering logic for the M5 manager
// (M5 install doc §1 "Manager behavior (Note 1, made concrete)").
//
// The manager is the ONLY writer of ttl_s (M5 delivery plan WP-5.1):
//	ttl_s = max(3 × B1 boot-time p50, brief estimate)
// The B1 p50 is configurable via BOOT_P50_S (sane default below); the brief
// estimate arrives in the gateway request (ttl_s) or falls back to
// BRIEF_ESTIMATE_S. Absent ttl_s ⇒ M2 applies POC_TTL_DEFAULT (900s); the
// reaper is authoritative either way.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Env var names (M5 compose block + TTL sizing).
const (
	EnvRabbitMQURL     = "RABBITMQ_URL"
	EnvAgentListRepo   = "AGENT_LIST_REPO"
	EnvGitHubReadToken = "GITHUB_READ_TOKEN"
	EnvTasksExchange   = "TASKS_EXCHANGE"   // agent.tasks (topic)
	EnvResponseQueue   = "RESPONSE_QUEUE"   // agent.tasks.response (default exchange)
	EnvReportsDir      = "REPORTS_DIR"      // /reports
	EnvBootP50S        = "BOOT_P50_S"       // M3 B1 boot-time p50, seconds
	EnvBriefEstimateS  = "BRIEF_ESTIMATE_S" // default brief estimate, seconds
	EnvRepoTarget      = "REPO_TARGET"      // project repo the coder writes to
	EnvRequester       = "REQUESTER"        // who asked (response path)
	EnvListen          = "LISTEN"           // manager API, default 0.0.0.0:8086
)

// Sane defaults for the configurable TTL inputs.
const (
	DefaultBootP50S   = 45  // placeholder p50 until M3's B1 evidence is measured; override via BOOT_P50_S
	DefaultBriefEstS  = 900 // matches M2's POC_TTL_DEFAULT as the default estimate
	TTLBootMultiplier = 3   // max(3 × B1 p50, estimate)
)

// TaskMessage mirrors schemas/message-v1.json (required: task_id UUID,
// agent_ref, behavioral_spec, repo_target, requester; optional: ttl_s,
// manager-only). additionalProperties=false: nothing else may be sent.
type TaskMessage struct {
	TaskID         string `json:"task_id"`
	AgentRef       string `json:"agent_ref"`
	BehavioralSpec string `json:"behavioral_spec"`
	RepoTarget     string `json:"repo_target"`
	Requester      string `json:"requester"`
	TTLS           *int64 `json:"ttl_s,omitempty"`
}

// AgentIndexEntry is one row of the Available Agent List's index.json
// (M3 doc §1a: [{ref, name, role, folder, skills[], spec_ref}]).
type AgentIndexEntry struct {
	Ref     string   `json:"ref"`
	Name    string   `json:"name"`
	Role    string   `json:"role"`
	Folder  string   `json:"folder"`
	Skills  []string `json:"skills"`
	SpecRef string   `json:"spec_ref"`
}

// TaskRequest is what the gateway forwards: {title, agent_capability,
// task_brief} plus an optional ttl_s the engineer supplies as the brief
// estimate (M5 install.sh F4 exercises the short-estimate path this way).
type TaskRequest struct {
	Title           string `json:"title"`
	AgentCapability string `json:"agent_capability"`
	TaskBrief       string `json:"task_brief"`
	TTLS            int64  `json:"ttl_s,omitempty"` // brief estimate only; manager owns the final value
}

// TaskResponse is what the M2 worker publishes to agent.tasks.response
// (M2 doc: {task_id, status, artifact}); correlated on task_id.
type TaskResponse struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Artifact string `json:"artifact"`
}

// HumanReport is the /reports/<task_id>.json shape (M5 §2 report contract:
// outcome + artifact pointer, e.g. task/<id> branch / commit; failure
// reports carry a human-visible reason).
type HumanReport struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"` // success | failure
	Outcome     string `json:"outcome"`
	Artifact    string `json:"artifact,omitempty"`
	Reason      string `json:"reason,omitempty"`
	CompletedAt string `json:"completed_at"`
}

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// ComputeTTLS applies the manager-only ttl_s rule:
// max(TTLBootMultiplier × bootP50S, briefEstimate), floored at 1 (schema minimum).
func ComputeTTLS(bootP50S, briefEstimate int64) int64 {
	if bootP50S < 0 {
		bootP50S = 0
	}
	ttl := TTLBootMultiplier * bootP50S
	if briefEstimate > ttl {
		ttl = briefEstimate
	}
	if ttl < 1 {
		ttl = 1
	}
	return ttl
}

// ComposeTaskMessage builds the M1 task message per schemas/message-v1.json:
// a fresh task_id (UUID, the single idempotency key), the agent_ref built
// from the repo location + the agent's folder (Note 1), the behavioral spec,
// the project repo target, the requester, and the manager-owned ttl_s.
// behaviorSpec is the spec the manager located in the clone (the
// behavioral-spec.md content, or the index's spec_ref when unreadable).
func ComposeTaskMessage(req TaskRequest, agent AgentIndexEntry, catalogRepoURL, repoTarget, requester string, bootP50S, briefEstimate int64) (TaskMessage, error) {
	if agent.Folder == "" {
		return TaskMessage{}, fmt.Errorf("agent index entry has no folder")
	}
	if catalogRepoURL == "" {
		return TaskMessage{}, fmt.Errorf("catalog repo URL empty — cannot build agent_ref")
	}
	if repoTarget == "" {
		return TaskMessage{}, fmt.Errorf("repo_target empty — set REPO_TARGET")
	}
	if requester == "" {
		return TaskMessage{}, fmt.Errorf("requester empty — set REQUESTER")
	}
	ttl := ComputeTTLS(bootP50S, briefEstimate)
	msg := TaskMessage{
		TaskID:         newUUID(),
		AgentRef:       fmt.Sprintf("%s#%s", catalogRepoURL, agent.Folder),
		BehavioralSpec: agent.SpecRef,
		RepoTarget:     repoTarget,
		Requester:      requester,
		TTLS:           &ttl,
	}
	return msg, msg.Validate()
}

// Validate checks schema conformance locally (the M1 adapter re-validates
// against schemas/message-v1.json; the manager pre-checks the same rules so
// a misconfigured manager fails at compose time, not at the DLQ).
func (m TaskMessage) Validate() error {
	if !uuidRe.MatchString(m.TaskID) {
		return fmt.Errorf("task_id not a UUID: %q", m.TaskID)
	}
	for name, v := range map[string]string{
		"agent_ref":       m.AgentRef,
		"behavioral_spec": m.BehavioralSpec,
		"repo_target":     m.RepoTarget,
		"requester":       m.Requester,
	} {
		if v == "" {
			return fmt.Errorf("required field %s empty", name)
		}
	}
	if m.TTLS != nil && *m.TTLS < 1 {
		return fmt.Errorf("ttl_s below schema minimum 1")
	}
	return nil
}

// MarshalStrict renders the message with omitempty so no schema-forbidden
// extra properties can appear (additionalProperties=false).
func (m TaskMessage) MarshalStrict() ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var probe map[string]any
	if err := json.Unmarshal(b, &probe); err != nil {
		return nil, err
	}
	allowed := map[string]bool{
		"task_id": true, "agent_ref": true, "behavioral_spec": true,
		"repo_target": true, "requester": true, "ttl_s": true,
	}
	for k := range probe {
		if !allowed[k] {
			return nil, fmt.Errorf("field %q not allowed by schemas/message-v1.json", k)
		}
	}
	return b, nil
}

// RoutingKey is the D-routing rule: agent.tasks.<capability> on the
// agent.tasks topic exchange.
func RoutingKey(capability string) string {
	return "agent.tasks." + capability
}

// MatchAgent searches the agent index for the request's role/capability
// (M5 §1 step 2). First role match wins; a capability naming a specific
// agent ref also matches.
func MatchAgent(index []AgentIndexEntry, capability string) (AgentIndexEntry, error) {
	if capability == "" {
		return AgentIndexEntry{}, fmt.Errorf("empty agent_capability")
	}
	for _, e := range index {
		if e.Role == capability {
			return e, nil
		}
	}
	for _, e := range index {
		if e.Ref == capability {
			return e, nil
		}
	}
	return AgentIndexEntry{}, fmt.Errorf("no agent in the index matches capability %q", capability)
}

// ParseAgentIndex decodes the clone's index.json.
func ParseAgentIndex(data []byte) ([]AgentIndexEntry, error) {
	var idx []AgentIndexEntry
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("index.json: %w", err)
	}
	if len(idx) == 0 {
		return nil, fmt.Errorf("index.json is empty")
	}
	return idx, nil
}

// ResponseMatches is the response-queue filter: consume agent.tasks.response,
// act only on the message whose task_id equals the task this manager
// published (task_id is the single request/response identifier — no
// separate correlation_id). Unknown/blank task_ids are ignored.
func ResponseMatches(resp TaskResponse, taskID string) bool {
	return resp.TaskID != "" && resp.TaskID == taskID
}

// BuildReport turns the worker's response into the human report written to
// /reports/<task_id>.json (outcome + artifact pointer; failure carries the
// reason the engineer can see).
func BuildReport(resp TaskResponse, reason string) (HumanReport, error) {
	if resp.TaskID == "" {
		return HumanReport{}, fmt.Errorf("response has no task_id")
	}
	status := resp.Status
	if status != "success" && status != "failure" {
		return HumanReport{}, fmt.Errorf("unknown status %q", resp.Status)
	}
	r := HumanReport{
		TaskID:      resp.TaskID,
		Status:      status,
		Outcome:     fmt.Sprintf("task %s %s", resp.TaskID, status),
		Artifact:    resp.Artifact,
		Reason:      reason,
		CompletedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if status == "failure" && r.Reason == "" {
		r.Reason = "task failed (no reason supplied by the worker)"
	}
	return r, nil
}

// newUUID is a var so tests can pin task_id deterministically.
var newUUID = defaultUUID
