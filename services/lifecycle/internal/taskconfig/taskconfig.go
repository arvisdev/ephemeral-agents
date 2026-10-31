// Package taskconfig defines the per-run task configuration carried in the
// M1 message (schemas/message-v1.json) and the M2 response payload published
// to agent.tasks.response. Pure logic lives here so it is unit-testable
// without Docker, Temporal, or RabbitMQ.
package taskconfig

import (
	"errors"
	"fmt"
	"time"
)

// Lifecycle statuses for a task run (M2 delivery plan state machine:
// creating -> running -> (complete|failed|timed-out) -> reaped).
const (
	StatusComplete = "complete"
	StatusFailed   = "failed"
	StatusTimedOut = "timed-out"
)

// TaskConfig is the per-run config the Temporal workflow passes to the
// executes_activity activity. JSON shape matches schemas/message-v1.json.
type TaskConfig struct {
	TaskID         string `json:"task_id"`
	AgentRef       string `json:"agent_ref"`
	BehavioralSpec string `json:"behavioral_spec"`
	RepoTarget     string `json:"repo_target"`
	Requester      string `json:"requester"`
	// TTLS is optional: when 0 the lifecycle applies POC_TTL_DEFAULT.
	// The reaper is authoritative either way (R1 backstop).
	TTLS int64 `json:"ttl_s,omitempty"`
}

// Validate enforces the required fields from message-v1.json.
func (c TaskConfig) Validate() error {
	if c.TaskID == "" {
		return errors.New("taskconfig: task_id is required")
	}
	if c.AgentRef == "" {
		return errors.New("taskconfig: agent_ref is required")
	}
	if c.BehavioralSpec == "" {
		return errors.New("taskconfig: behavioral_spec is required")
	}
	if c.RepoTarget == "" {
		return errors.New("taskconfig: repo_target is required")
	}
	if c.Requester == "" {
		return errors.New("taskconfig: requester is required")
	}
	return nil
}

// EffectiveTTL returns the owner-settled hybrid: ttl_s from the message when
// present, otherwise the lifecycle config default (POC_TTL_DEFAULT).
func (c TaskConfig) EffectiveTTL(defaultTTL time.Duration) time.Duration {
	if c.TTLS > 0 {
		return time.Duration(c.TTLS) * time.Second
	}
	return defaultTTL
}

// ExpiryTime is the absolute moment the reaper becomes authoritative.
func (c TaskConfig) ExpiryTime(defaultTTL time.Duration, start time.Time) time.Time {
	return start.Add(c.EffectiveTTL(defaultTTL))
}

// Response is the payload published to the agent.tasks.response queue on
// completion (success or failure) — exactly {task_id, status, artifact}.
type Response struct {
	TaskID   string `json:"task_id"`
	Status   string `json:"status"`
	Artifact string `json:"artifact"`
}

// Response builds the correlated response for this task. artifact is the
// per-task log path captured by the lifecycle service.
func (c TaskConfig) Response(status, artifact string) Response {
	return Response{TaskID: c.TaskID, Status: status, Artifact: artifact}
}

// SpawnRequest is the body the worker POSTs to the lifecycle /spawn endpoint.
type SpawnRequest struct {
	TaskConfig
}

// SpawnResponse is returned by /spawn: the container ID plus the absolute
// expiry (RFC3339) the worker polls against. The lifecycle computes the TTL
// so worker and reaper agree on one authoritative value.
type SpawnResponse struct {
	ContainerID string `json:"container_id"`
	Expires     string `json:"expires"` // RFC3339
}

// StatusResponse is returned by GET /status?task=<id>.
type StatusResponse struct {
	TaskID      string `json:"task_id"`
	ContainerID string `json:"container_id"`
	State       string `json:"state"` // created|running|exited|gone
	ExitCode    int    `json:"exit_code"`
	Expires     string `json:"expires"`
}

// ParseExpires reads an RFC3339 poc.expires label/field.
func ParseExpires(s string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("taskconfig: bad expires %q: %w", s, err)
	}
	return t, nil
}
