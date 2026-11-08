// Copyright 2026 A&R Visual Development Inc (arvisdev)
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command worker is the M2 Temporal worker: registers the TaskWorkflow
// workflow (M1 — one task message -> one executes_activity invocation with
// RetryOptions{MaximumAttempts: 2}) and the executes_activity activity on the
// default namespace. The activity spawns an ephemeral
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
// Retry policy note: Temporal Go sets activity RetryOptions workflow-side —
// TaskWorkflow (internal/workflow) schedules executes_activity with
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
	workflowpkg "github.com/ephemeral-agents/poc/services/lifecycle/internal/workflow"
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
	// TaskWorkflow: workflow name defaults to the function name — exactly the
	// type the M1 topic-bridge starts (WorkflowID = task_id).
	w.RegisterWorkflow(workflowpkg.TaskWorkflow)
	w.RegisterActivityWithOptions(act.Execute, sdkactivity.RegisterOptions{Name: "executes_activity"})

	log.Info("worker starting", "temporal", temporalAddr, "task_queue", taskQueue, "lifecycle_api", apiBase)
	if err := w.Run(nil); err != nil {
		log.Error("worker exited", "err", err)
		os.Exit(1)
	}
}
