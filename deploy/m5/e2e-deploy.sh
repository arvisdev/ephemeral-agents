#!/usr/bin/env bash
# =============================================================================
# M5 — ordered full-stack bring-up (stage-gated S1–S7)
# =============================================================================
# Source: version 1/installation-deployment/M5-End-to-End-Installation-and-Deployment.md §3
# Idempotent; each stage's healthcheck must pass before the next; any failure
# halts + prints the failing stage (which names the owning milestone's gate
# test — recovery never means re-running the chain).
#
# Usage: bash deploy/m5/e2e-deploy.sh
# Env: VM_XA VM_XB VM_3 (ssh aliases), ASSUME_YES=1 for non-interactive
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
VM_XA="${VM_XA:-vm-xa}"; VM_XB="${VM_XB:-vm-xb}"; VM_3="${VM_3:-vm-hindsight}"
FAILED_STAGE=""

stage() { # stage <name> <cmd...> — run; on failure halt with the stage named
  local name="$1"; shift
  echo "=== S: $name ==="
  if "$@"; then
    echo "--- $name : PASS"
  else
    echo "--- $name : FAIL"
    FAILED_STAGE="$name"
    exit 1
  fi
}

# S1 chassis: M0 install.sh (idempotent re-run)
stage "S1 chassis (owning gate: TP-M0)" \
  bash "$REPO_ROOT/deploy/m0/install.sh" --no-gate

# S2 adapter: M1 compose up + consumer-healthy check
stage "S2 adapter (owning gate: M1 §4 / TP-M1)" \
  ssh "$VM_XB" "cd /opt/poc/deploy/m1 && docker compose up -d topic-bridge"
stage "S2 adapter consumer healthy" \
  ssh "$VM_XB" "docker logs m1-topic-bridge-1 2>&1 | grep -qiE 'consumer|consuming'"

# S3 lifecycle: M2 compose up + deploy gate (pinned images present) + worker registered
stage "S3 lifecycle deploy gate (pinned image exists — no-pull rule)" \
  ssh "$VM_XB" "docker image inspect poc/executor-stub:1.0.0 >/dev/null"
stage "S3 lifecycle compose up" \
  ssh "$VM_XB" "cd /opt/poc/deploy/m2 && docker compose up -d"
stage "S3 lifecycle healthy" \
  ssh "$VM_XB" "docker exec m2-lifecycle-1 curl -sf http://127.0.0.1:8090/health"
stage "S3 worker registered (polling temporal)" \
  ssh "$VM_XB" "docker logs m2-worker-1 2>&1 | grep -qiE 'WorkflowType TaskWorkflow|worker started|registered'"

# S4 executors: M3 images load-verified on vm-xb; token issuance dry-run
stage "S4 executor images present on vm-xb" \
  ssh "$VM_XB" "docker image inspect poc/hermes-ephemeral:1.0.0 >/dev/null && docker image inspect poc/claude-coder:1.0.0 >/dev/null"
stage "S4 token issuance dry-run (issue + verify + revoke)" \
  ssh "$VM_XB" "set -a; . /opt/poc/secrets/m3.env; set +a; bash /opt/poc/deploy/m3/issue-token.sh --dry-run"

# S5 memory: M4 hindsight compose up on vm-hindsight + /healthz check
# (the llm-wiki is a git repo cloned by the executor at boot — no service here)
stage "S5 hindsight compose up (vm-hindsight)" \
  ssh "$VM_3" "cd /opt/poc/deploy/m4/hindsight && docker compose up -d"
stage "S5 hindsight /healthz" \
  ssh "$VM_3" "curl -sf http://127.0.0.1:8081/healthz"

# S6 front-house: M5 compose up on vm-xa + gateway /healthz
stage "S6 front-house compose up (vm-xa)" \
  ssh "$VM_XA" "cd /opt/poc/deploy/m5 && docker compose up -d"
stage "S6 gateway /healthz" \
  ssh "$VM_XA" "curl -sf http://127.0.0.1:8080/healthz"

# S7 smoke: one M2-level task with the stub executor (proves S2–S3 before real agents)
stage "S7 smoke task (stub executor through the full flow)" \
  bash "$REPO_ROOT/deploy/m2/tests/run-task.sh" "{\"task_id\":\"s7-smoke-$(date +%s)\",\"executor\":\"stub\",\"agent_ref\":\"ephemeral-org/eph-backend-engineer\",\"agent_repo_url\":\"https://github.com/<organization>/agent-roster\",\"repo_folder\":\"ephemeral-org/eph-backend-engineer\",\"repo_target\":\"repos/ephemeral-poc-demo\",\"task_prompt\":\"S7 smoke: confirm the flow\",\"max_turns\":3,\"behavioral_spec\":\"S7 end-to-end smoke\",\"requester\":\"m5-s7\",\"ttl_s\":300}"

echo "=== e2e-deploy: all stages S1–S7 PASS ==="
