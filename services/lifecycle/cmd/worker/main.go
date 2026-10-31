// Command worker is the M2 Temporal worker: registers the executes_activity
// activity on the default namespace. The activity spawns an ephemeral
// executor via the lifecycle API, polls until complete/failed/TTL, reaps,
// publishes {task_id, status, artifact} to agent.tasks.response, and returns.
//
// Config (env, per deploy/m2/compose.yaml):
//
//	TEMPORAL_ADDR=temporal:7233
//	LIFECYCLE_API=http://lifecycle:8090
//	RABBITMQ_URL=amqp://poc:***@rabbitmq:5672/
//	POC_TTL_DEFAULT=900s      (worker-side fallback; the lifecycle TTL is authoritative)
//	POC_POLL_INTERVAL=2s
//
// Retry policy note: Temporal Go sets activity RetryOptions workflow-side
// (M1's TaskWorkflow). The M1 workflow should schedule executes_activity with
// RetryOptions{MaximumAttempts: 2} and a StartToClose timeout derived from
// the task's ttl_s (plus spawn/publish headroom); the reaper backstop owns
// any container whose activity is lost.
package main

import (
	"log/slog"
	"os"
	"time"

	sdkactivity "go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/activity"
	"github.com/ephemeral-agents/poc/services/lifecycle/internal/apiclient"
	"github.com/ephemeral-agents/poc/services/lifecycle/internal/publisher"
)

const taskQueue = "poc-tasks"

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustDuration(key, def string) time.Duration {
	d, err := time.ParseDuration(envOr(key, def))
	if err != nil || d <= 0 {
		slog.Error("bad duration env", "key", key, "value", envOr(key, def))
		os.Exit(1)
	}
	return d
}

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	temporalAddr := envOr("TEMPORAL_ADDR", "temporal:7233")
	apiBase := envOr("LIFECYCLE_API", "http://lifecycle:8090")
	rabbitURL := envOr("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	defaultTTL := mustDuration("POC_TTL_DEFAULT", "900s")
	pollEvery := mustDuration("POC_POLL_INTERVAL", "2s")

	c, err := client.Dial(client.Options{HostPort: temporalAddr, Namespace: "default"})
	if err != nil {
		log.Error("temporal dial", "addr", temporalAddr, "err", err)
		os.Exit(1)
	}
	defer c.Close()

	act := &activity.Activity{
		API:        apiclient.New(apiBase),
		Publisher:  publisher.New(rabbitURL),
		Log:        log,
		PollEvery:  pollEvery,
		DefaultTTL: defaultTTL,
	}

	w := worker.New(c, taskQueue, worker.Options{})
	w.RegisterActivityWithOptions(act.Execute, sdkactivity.RegisterOptions{Name: "executes_activity"})

	log.Info("worker starting", "temporal", temporalAddr, "task_queue", taskQueue, "lifecycle_api", apiBase)
	if err := w.Run(nil); err != nil {
		log.Error("worker exited", "err", err)
		os.Exit(1)
	}
}
