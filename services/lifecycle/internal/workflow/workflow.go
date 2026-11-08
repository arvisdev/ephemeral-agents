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

// Package workflow implements the M1 Temporal workflow TaskWorkflow: one
// validated task message (schemas/message-v1.json) -> one executes_activity
// invocation. The topic-bridge starts this workflow with WorkflowID = task_id
// (the idempotency key); the M2 worker registers both this workflow and the
// executes_activity activity on task queue "poc-tasks".
package workflow

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/ephemeral-agents/poc/services/lifecycle/internal/taskconfig"
)

// defaultWorkflowTTL is the workflow-side fallback when the message carries no
// ttl_s: POC_TTL_DEFAULT (900s, M2 compose). The lifecycle-side TTL plus the
// label-based reaper remain authoritative for the container either way (R1).
const defaultWorkflowTTL = 900 * time.Second

// headroom covers spawn + status-poll + response-publish beyond the TTL.
const headroom = 2 * time.Minute

// TaskWorkflow schedules executes_activity exactly once per task message with
// the retry policy the M1 doc specifies (MaximumAttempts: 2) and a
// StartToClose timeout derived from the task's ttl_s plus headroom.
func TaskWorkflow(ctx workflow.Context, cfg taskconfig.TaskConfig) (taskconfig.Response, error) {
	logger := workflow.GetLogger(ctx)

	ttl := time.Duration(cfg.TTLS) * time.Second
	if ttl <= 0 {
		ttl = defaultWorkflowTTL
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: ttl + headroom,
		RetryPolicy:         &temporal.RetryPolicy{MaximumAttempts: 2},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var resp taskconfig.Response
	if err := workflow.ExecuteActivity(ctx, "executes_activity", cfg).Get(ctx, &resp); err != nil {
		logger.Error("task workflow failed", "task_id", cfg.TaskID, "err", err)
		return taskconfig.Response{}, err
	}
	logger.Info("task workflow complete", "task_id", resp.TaskID, "status", resp.Status, "artifact", resp.Artifact)
	return resp, nil
}
