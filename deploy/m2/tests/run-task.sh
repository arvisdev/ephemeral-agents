#!/usr/bin/env bash
# =============================================================================
# M2 test driver — run one TaskWorkflow end-to-end
# =============================================================================
# Implements M2 install doc §3: "Driver: deploy/m2/tests/run-task.sh '<taskJson>'
# → starts a TaskWorkflow via Temporal CLI and waits for completion."
#
# Usage: run-task.sh '<json>'
#   json: at least {"task_id":"..."}; optional ttl_s, inject (exit-1|hang)
#
# Behavior: starts the workflow on the M1/M2 task queue via `temporal workflow
# start` (on vm-xb, or locally if TEMPORAL_HOST is set), polls until CLOSED,
# prints the outcome (success | fail | timeout) and verifies the reaper left
# no container labeled poc.task=<task_id>. Exit 0 on success, non-zero else.
#
# Env:
#   VM_XB        ssh alias for vm-xb (default vm-xb) — where temporal CLI runs
#   TEMPORAL     temporal CLI binary (default temporal)
#   TASK_QUEUE   default poc-tasks
#   TIMEOUT_S    poll cap (default = task ttl_s + 60, min 90)
set -euo pipefail

VM_XB="${VM_XB:-vm-xb}"
TEMPORAL="${TEMPORAL:-temporal}"
TASK_QUEUE="${TASK_QUEUE:-poc-tasks}"

[ -n "${1:-}" ] || { echo "usage: run-task.sh '<taskJson>'" >&2; exit 64; }
TASK_JSON="$1"

t_field() { printf '%s' "$TASK_JSON" | python3 -c "import json,sys;print(json.load(sys.stdin).get('$1',''))"; }
TASK_ID="$(t_field task_id)"
[ -n "$TASK_ID" ] || { echo "task json needs task_id" >&2; exit 64; }
TTL_S="$(t_field ttl_s)"; TTL_S="${TTL_S:-900}"
INJECT="$(t_field inject)"

TIMEOUT_S="${TIMEOUT_S:-$(( TTL_S + 60 ))}"; [ "$TIMEOUT_S" -ge 90 ] || TIMEOUT_S=90

# FIX (run5): non-interactive ssh shells do not source the interactive PATH
# (~/.local/bin holds the temporal CLI) — prepend it for every remote call.
run_ssh() { if [ "$VM_XB" = "local" ]; then bash -c "$*"; else ssh "$VM_XB" "PATH=\"\$HOME/.local/bin:\$PATH\"; export PATH; $*"; fi; }

echo "{\"event\":\"start\",\"task_id\":\"$TASK_ID\",\"inject\":\"${INJECT:-none}\",\"ttl_s\":$TTL_S}"

# start the workflow (workflow ID = task_id — the M1 idempotency key)
run_ssh "$TEMPORAL workflow start \
  --task-queue '$TASK_QUEUE' --type TaskWorkflow \
  --workflow-id '$TASK_ID' --input '$TASK_JSON'" \
  || { echo "{\"task_id\":\"$TASK_ID\",\"status\":\"fail\",\"reason\":\"workflow start failed\"}"; exit 1; }

# poll via `workflow result` — blocking and lag-free (workflow list lags 10-40s)
# FIX (run5 root cause): the list-based poller timed out on completed workflows
# because the list view lagged; result waits for the true completion.
res="$(run_ssh "timeout $TIMEOUT_S $TEMPORAL workflow result --workflow-id '$TASK_ID'" 2>/dev/null)" || res=""
case "$res" in
  *'"status":"complete"'*) echo "{\"task_id\":\"$TASK_ID\",\"status\":\"success\"}"; rc=0 ;;
  *'"status":"failed"'*)    echo "{\"task_id\":\"$TASK_ID\",\"status\":\"fail\"}";      rc=1 ;;
  *'"status":"timed-out"'*) echo "{\"task_id\":\"$TASK_ID\",\"status\":\"timeout\",\"note\":\"reaper path (M2 V3)\"}"; rc=1 ;;
  *COMPLETED*) echo "{\"task_id\":\"$TASK_ID\",\"status\":\"success\"}"; rc=0 ;;
  "") echo "{\"task_id\":\"$TASK_ID\",\"status\":\"timeout\",\"note\":\"no result within TIMEOUT_S (reaper path)\"}"; rc=1 ;;
  *) echo "{\"task_id\":\"$TASK_ID\",\"status\":\"fail\"}"; rc=1 ;;
esac

# teardown assertion: the reaper must have removed the task container
leftover="$(run_ssh "docker ps -a --filter label=poc.task='$TASK_ID' -q" || true)"
if [ -n "$leftover" ]; then
  echo "{\"task_id\":\"$TASK_ID\",\"reap\":\"FAIL\",\"survivor\":\"$leftover\"}"
  rc=1
else
  echo "{\"task_id\":\"$TASK_ID\",\"reap\":\"ok\",\"container\":\"gone\"}"
fi
exit "$rc"
