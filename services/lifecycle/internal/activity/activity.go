// Package activity implements the Temporal activity executes_activity:
// spawn → poll status until complete/failed/TTL → reap → publish result.
package activity

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// ResponsePublisher publishes {task_id, status, artifact} to the
// agent.tasks.response queue via the RabbitMQ DEFAULT exchange (empty
// exchange name, queue name as routing key — M0 declares it unbound).
type ResponsePublisher interface {
	PublishResponse(ctx context.Context, r taskconfig.Response) error
}

// LifecycleAPI is the boundary the worker's activity uses to reach the
// lifecycle service; *apiclient.Client implements it, tests fake it.
type LifecycleAPI interface {
	Spawn(ctx context.Context, cfg taskconfig.TaskConfig) (taskconfig.SpawnResponse, error)
	Status(ctx context.Context, taskID string) (taskconfig.StatusResponse, error)
	Reap(ctx context.Context, taskID, reason string) error
}

// Activity drives one ephemeral executor container end to end.
type Activity struct {
	API        LifecycleAPI
	Publisher  ResponsePublisher
	Log        *slog.Logger
	PollEvery  time.Duration // POC_POLL_INTERVAL
	DefaultTTL time.Duration // fallback when the lifecycle didn't return an expiry
	TTLGrace   time.Duration // extra wait past expiry before declaring timed-out
}

// Execute is the activity body registered as "executes_activity".
func (a *Activity) Execute(ctx context.Context, cfg taskconfig.TaskConfig) (taskconfig.Response, error) {
	if err := cfg.Validate(); err != nil {
		return taskconfig.Response{}, err
	}
	spawn, err := a.API.Spawn(ctx, cfg)
	if err != nil {
		return taskconfig.Response{}, fmt.Errorf("spawn: %w", err)
	}
	expires, _ := taskconfig.ParseExpires(spawn.Expires)
	if expires.IsZero() {
		expires = cfg.ExpiryTime(a.DefaultTTL, time.Now())
	}
	a.Log.Info("spawned", "task_id", cfg.TaskID, "container_id", spawn.ContainerID, "expires", spawn.Expires)

	status, reason := a.pollUntilTerminal(ctx, cfg.TaskID, expires)
	// Teardown runs on success, failure, and timeout — always.
	if err := a.API.Reap(ctx, cfg.TaskID, reason); err != nil {
		a.Log.Warn("reap failed (reaper backstop will retry)", "task_id", cfg.TaskID, "err", err)
	}
	resp := cfg.Response(status, fmt.Sprintf("/var/poc/tasks/%s.log", cfg.TaskID))
	if pubErr := a.Publisher.PublishResponse(ctx, resp); pubErr != nil {
		// Activity error → Temporal retries (retry policy); reap is idempotent.
		return taskconfig.Response{}, fmt.Errorf("publish response: %w (task %s %s)", pubErr, resp.TaskID, resp.Status)
	}
	return resp, nil
}

// pollUntilTerminal polls /status until the container exits (complete/failed)
// or the authoritative expiry passes (timed-out). Context cancellation
// (activity timeout) surfaces as an error — Temporal retries once, and the
// reaper backstop owns the container either way.
func (a *Activity) pollUntilTerminal(ctx context.Context, taskID string, expires time.Time) (status, reason string) {
	poll := a.PollEvery
	if poll <= 0 {
		poll = 2 * time.Second
	}
	grace := a.TTLGrace
	if grace <= 0 {
		grace = 2 * time.Second
	}
	t := time.NewTicker(poll)
	defer t.Stop()
	deadline := expires.Add(grace)
	for {
		select {
		case <-ctx.Done():
			return taskconfig.StatusTimedOut, "activity-context-done"
		case <-t.C:
			st, err := a.API.Status(ctx, taskID)
			if err != nil {
				a.Log.Warn("status poll failed", "task_id", taskID, "err", err)
				continue
			}
			switch st.State {
			case "exited":
				if st.ExitCode == 0 {
					return taskconfig.StatusComplete, "exited-0"
				}
				return taskconfig.StatusFailed, fmt.Sprintf("exited-%d", st.ExitCode)
			case "gone":
				// Reaped externally (backstop) — treat as timed-out outcome.
				return taskconfig.StatusTimedOut, "container-gone"
			}
			if !time.Now().Before(deadline) {
				return taskconfig.StatusTimedOut, "ttl"
			}
		}
	}
}
