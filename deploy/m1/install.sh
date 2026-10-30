#!/usr/bin/env bash
# =============================================================================
# M1 — RABBITMQ → TEMPORAL ADAPTER: complete installation script
# =============================================================================
# Implements §0–§4 of:
#   version 1/installation-deployment/M1-Adapter-Installation-and-Deployment.md
# Gate afterwards:
#   version 1/test procedures/M1-Adapter-Test-Procedure.md  (TP-M1 must PASS
#   before the M2 installation may begin)
#
# Steps:
#   §0  Prereq checks (M0: queue exists, Temporal healthy)
#   §1  Language-decision check (adapter is Go — mandatory)
#   §2  Build + load poc/topic-bridge:1.0.0 onto vm-xb
#   §3  Deploy via compose + consumer-consume check
#   §4  Verification run: T1..T5 (the M1 gate tests) -> deploy/m1/evidence/
#
# Usage:  bash deploy/m1/install.sh [--check-only] [--no-gate]
#   ASSUME_YES=1  — skip interactive confirms
#
# Configuration (env, or deploy/m1/hosts.env / deploy/m0/hosts.env — auto-loaded):
#   VM_XB (ssh alias for vm-xb, default "vm-xb")
#   TEMPORAL_ADDR (default "vm-xb:7233")
#   M0_DONE=1   — skip §0 prereq checks (M0 gate already signed)
#
# Assets required in the repo (produced by the M1 delivery plan; checked first):
#   services/topic-bridge/cmd/adapter/main.go   (Go + Temporal Go SDK + amqp091-go)
#   services/topic-bridge/schemas/message-v1.json
#   deploy/m1/Dockerfile   deploy/m1/compose.yaml
#   deploy/m1/tests/publish.sh   deploy/m1/tests/assert.sh
#
# Rollback (doc §5): cd deploy/m1 && docker compose down topic-bridge  — stateless.
#
# FOR THE OPERATOR (junior dev) — read this before running:
#   1. The M0 gate must be signed (TP-M0 PASS). The script probes it in §0;
#      set M0_DONE=1 to skip the probes when you know the chassis is green.
#   2. `bash deploy/m1/install.sh --check-only`   # preview: no changes
#   3. `bash deploy/m1/install.sh`                # build -> load -> deploy ->
#      gate tests T1..T5, results in deploy/m1/evidence/m1-tests.txt
#   4. A gate-test failure: fix per M1 install doc §6, re-run from the failed
#      test. All T1..T5 PASS -> run TP-M1 before starting M2.
#
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
M1="$REPO_ROOT/deploy/m1"
EVIDENCE="$M1/evidence"
LOG="$EVIDENCE/install-m1.log"
mkdir -p "$EVIDENCE"

CHECK_ONLY=0; NO_GATE=0
ASSUME_YES="${ASSUME_YES:-0}"
for arg in "$@"; do
  case "$arg" in
    --check-only) CHECK_ONLY=1 ;;
    --no-gate) NO_GATE=1 ;;
    -h|--help) sed 's/^# \{0,1\}//' "$0"; exit 0 ;;
    *) echo "unknown option: $arg"; exit 64 ;;
  esac
done

# host map: m0's hosts.env is the canonical one; m1 may add overrides
[ -f "$REPO_ROOT/deploy/m0/hosts.env" ] && { set -a; . "$REPO_ROOT/deploy/m0/hosts.env"; set +a; }
[ -f "$M1/hosts.env" ] && { set -a; . "$M1/hosts.env"; set +a; }
VM_XB="${VM_XB:-vm-xb}"
TEMPORAL_ADDR="${TEMPORAL_ADDR:-$VM_XB:7233}"
IMAGE_TAG="${IMAGE_TAG:-1.0.0}"

log()  { printf '[m1 %s] %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG"; }
ok()   { log "OK   - $*"; }
warn() { log "WARN - $*"; }
die()  { log "FAIL: $*"; exit 1; }

confirm() {
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: would apply — $*"; return 0; fi
  if [ "$ASSUME_YES" = "1" ]; then log "assumed — $*"; else
    local ans=""
    read -r -p "  proceed? [y/N] " ans
    [ "$ans" = "y" ] || die "aborted by operator"
  fi
}

# ---------------------------------------------------------------- §0 prereqs
prereqs() {
  log "§0 — prereq checks (M0 gate)"
  [ "${M0_DONE:-0}" = "1" ] && { log "§0   M0_DONE=1 — skipping live prereq checks"; return 0; }
  command -v temporal >/dev/null || die "temporal CLI not found on build host (install per M0 §4)"
  temporal operator cluster health 2>/dev/null | grep -q "HEALTHY" \
    || die "Temporal not healthy (TEMPORAL_ADDR=$TEMPORAL_ADDR)"
  local envf="$REPO_ROOT/deploy/m0/rabbitmq/.env"
  [ -f "$envf" ] || die "deploy/m0/rabbitmq/.env missing — run M0 install first"
  local pass; pass="$(grep '^RABBITMQ_PASS=' "$envf" | cut -d= -f2)"
  local qmsg
  qmsg="$(printf 'user = "poc:%s"\n' "$pass" | ssh "$VM_XB" "curl -s -K - 'http://127.0.0.1:15672/api/queues/%2F/agent.tasks.durable' | grep -o '\"messages\":[0-9]*' || true")"
  log "§0   agent.tasks.durable queue present: ${qmsg:-<not found>}"
  if [ -z "$qmsg" ]; then
    die "agent.tasks.durable not found — RabbitMQ topology missing (re-run M0 install §3)"
  fi
  ok "§0 prereqs"
}

# ---------------------------------------------------------------- assets
check_assets() {
  local missing=() f
  for f in \
    "$REPO_ROOT/services/topic-bridge/cmd/adapter/main.go" \
    "$REPO_ROOT/services/topic-bridge/schemas/message-v1.json" \
    "$M1/Dockerfile" \
    "$M1/compose.yaml" \
    "$M1/tests/publish.sh" \
    "$M1/tests/assert.sh"
  do
    [ -e "$f" ] || missing+=("$f")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    if [ "$CHECK_ONLY" = "1" ]; then
      log "check-only: missing assets (not a failure in check-only):"
      printf '  %s\n' "${missing[@]}"
      exit 0
    fi
    log "missing assets (the M1 delivery plan produces these):"
    printf '  %s\n' "${missing[@]}"
    die "asset check failed — build the assets, then re-run"
  fi
  ok "assets present"
}

# ---------------------------------------------------------------- §1 language check
language_check() {
  log "§1 — language-decision check: the adapter MUST be Go (owner instruction: speed + size)"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip build"; return 0; fi
  local dir="$REPO_ROOT/services/topic-bridge"
  if [ ! -f "$dir/go.mod" ]; then
    confirm "initialize Go module in $dir (go mod init + go get sdk-go, amqp091-go)"
    if [ "$CHECK_ONLY" != "1" ]; then
      ( cd "$dir" && go mod init poc/topic-bridge \
        && go get github.com/temporalio/sdk-go@latest github.com/rabbitmq/amqp091-go@latest )
    fi
  fi
  log "§1   go vet + build (proof of Go toolchain, M0 §4b chain)"
  ( cd "$dir" && go vet ./... && go build ./... )
  ok "§1 adapter is Go"
}

# ---------------------------------------------------------------- §2 build + load
build_and_load() {
  log "§2 — build poc/topic-bridge:$IMAGE_TAG and load onto $VM_XB (no registry)"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip build"; return 0; fi
  docker build -f "$M1/Dockerfile" "$REPO_ROOT/services/topic-bridge" -t "poc/topic-bridge:$IMAGE_TAG"
  # language check on the BINARY (Go build metadata embedded in the ELF)
  docker run --rm --entrypoint sh "poc/topic-bridge:$IMAGE_TAG" \
    -c 'strings /usr/local/bin/adapter | grep -qm1 "Go build information" && echo GO' | grep -q GO \
    || die "adapter binary is not a Go build — language decision violated"
  docker save "poc/topic-bridge:$IMAGE_TAG" | ssh "$VM_XB" "docker load"
  ok "§2 image built (Go) + loaded"
}

# ---------------------------------------------------------------- §3 deploy
deploy_adapter() {
  log "§3 — deploy topic-bridge on $VM_XB"
  confirm "deploy topic-bridge on $VM_XB (compose up -d)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  local envf="$REPO_ROOT/deploy/m0/rabbitmq/.env"
  local pass; pass="$(grep '^RABBITMQ_PASS=' "$envf" | cut -d= -f2)"
  ssh "$VM_XB" "mkdir -p /opt/poc/deploy/m1"
  rsync -az "$M1/compose.yaml" "$VM_XB:/opt/poc/deploy/m1/compose.yaml"
  printf '%s\n' "RABBITMQ_PASS=$pass" | ssh "$VM_XB" 'umask 077; cat > /opt/poc/deploy/m1/.env && chmod 600 /opt/poc/deploy/m1/.env'  # stdin + 600: secret not in argv/logs
  # M0 services live in their own compose project on vm-xb -> shared external
  # network (record the network name in deploy/m1/.env notes / image-tags.env).
  ssh "$VM_XB" "cd /opt/poc/deploy/m1 && docker compose up -d topic-bridge"
  local i st=""
  for i in $(seq 1 30); do
    st="$(ssh "$VM_XB" "docker ps -f name=topic-bridge --format '{{.Status}}' || true")"
    case "$st" in *"Up"*) break ;; esac
    sleep 2
  done
  case "$st" in *"Up"*) : ;; *) die "topic-bridge not Up after 60s (M1 §6: shared compose network?)" ;; esac
  log "§3   consumer: $(ssh "$VM_XB" "docker logs topic-bridge 2>&1 | grep -iE 'consumer|consuming' | tail -3" || echo '<no consume line yet>')"
  ok "§3 topic-bridge deployed"
}

# ---------------------------------------------------------------- §4 gate tests
gate_tests() {
  log "§4 — verification run T1..T5 (the M1 gate tests) -> $EVIDENCE/m1-tests.txt"
  local out="$EVIDENCE/m1-tests.txt"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip gate tests"; return 0; fi
  local RMP; RMP="$(grep '^RABBITMQ_PASS=' "$REPO_ROOT/deploy/m0/rabbitmq/.env" | cut -d= -f2)"
  local count expected base delta ppid dlq
  {
    echo "# M1 gate tests — $(date -u)"
    wf_count() { temporal workflow list --type TaskWorkflow --limit 1000 2>/dev/null | grep -c "TaskWorkflow" || true; }
    # run-scoped: assert on the DELTA of workflows ever started (not absolute "running"
    # counts), so a re-run against leftover state doesn't false-fail.
    # T1: N -> N (N=50)
    base="$(wf_count)"
    bash "$M1/tests/publish.sh" 50
    sleep 5
    delta=$(( $(wf_count) - base ))
    echo "T1 n->n: +$delta started (expected 50) : $([ "$delta" -eq 50 ] && echo PASS || echo FAIL)"
    # T2: duplicate safety — republish all 50 (same task_ids -> zero new workflows)
    base="$(wf_count)"
    bash "$M1/tests/publish.sh" 50 --dup
    sleep 5
    delta=$(( $(wf_count) - base ))
    echo "T2 dup: +$delta started (expected 0) : $([ "$delta" -eq 0 ] && echo PASS || echo FAIL)"
    # T3: kill -9 mid-flight (all 20 redelivered, no doubles -> exactly +20)
    base="$(wf_count)"
    bash "$M1/tests/publish.sh" 20 & ppid=$!
    sleep 1
    ssh "$VM_XB" "docker kill topic-bridge && docker start topic-bridge"
    wait "$ppid" || true
    sleep 5
    delta=$(( $(wf_count) - base ))
    echo "T3 kill-9: +$delta started (expected 20, no doubles) : $([ "$delta" -eq 20 ] && echo PASS || echo FAIL)"
    # T4: poison pill
    bash "$M1/tests/publish.sh" --invalid 3
    sleep 5
    dlq="$(printf 'user = "poc:%s"\n' "$RMP" | ssh "$VM_XB" "curl -s -K - 'http://127.0.0.1:15672/api/queues/%2F/agent.tasks.dlq' | grep -o '\"messages\":[0-9]*' | cut -d: -f2 || echo 0")"
    echo "T4 dlq: $dlq (expected 3; consumer still up) : $([ "${dlq:-0}" -ge 3 ] && echo PASS || echo FAIL)"
    # T5: Temporal restart
    ssh "$VM_XB" "cd /opt/poc/deploy/m0/temporal && docker compose restart temporal"
    sleep 30
    base="$(wf_count)"
    bash "$M1/tests/publish.sh" 5
    sleep 5
    delta=$(( $(wf_count) - base ))
    echo "T5 temporal-restart: +$delta started (expected 5) : $([ "$delta" -eq 5 ] && echo PASS || echo FAIL)"
  } | tee "$out"
  if grep -q FAIL "$out"; then
    warn "M1 gate tests: FAILURES above — remediate per M1 §6 troubleshooting, re-run from the failed test."
    if [ "$NO_GATE" != "1" ]; then exit 1; fi
  else
    ok "M1 gate tests: T1..T5 PASS"
    log "Next: run TP-M1 — version 1/test procedures/M1-Adapter-Test-Procedure.md (file evidence in $EVIDENCE)."
  fi
}

# ---------------------------------------------------------------- main
main() {
  log "=== M1 adapter install start (check-only=$CHECK_ONLY no-gate=$NO_GATE) ==="
  prereqs
  check_assets
  confirm "M1: build + deploy + gate-test the Go adapter"
  language_check
  build_and_load
  deploy_adapter
  gate_tests
  log "=== M1 install complete ==="
}
main
