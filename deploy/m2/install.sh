#!/usr/bin/env bash
# =============================================================================
# M2 — CONTAINER LIFECYCLE MANAGER: complete installation script
# =============================================================================
# Implements §1–§4 of:
#   version 1/installation-deployment/M2-Lifecycle-Installation-and-Deployment.md
# Gate afterwards:
#   version 1/test procedures/M2-Lifecycle-Test-Procedure.md  (TP-M2 must PASS
#   before the M3 installation may begin)
#
# Steps:
#   §1  Build + load: poc/lifecycle:1.0.0, poc/worker:1.0.0, poc/executor-stub:1.0.0
#   §2  Deploy lifecycle + worker on vm-xb + no-pull deploy gate + /health
#   §3  Verification suite V1..V5 (the M2 gate tests; V4 = crash recovery,
#       the load-bearing test — failure carries the POC abort condition)
#   §4  Observability spot-checks (M2.5)
#
# Usage:  bash deploy/m2/install.sh [--check-only] [--no-gate]
#   ASSUME_YES=1  — skip interactive confirms
#
# Configuration (env, or deploy/m0/hosts.env — auto-loaded):
#   VM_XB (ssh alias for vm-xb, default "vm-xb")
#   M0_DONE=1 M1_DONE=1 — skip the prereq liveness probes
#
# Assets required in the repo (produced by the M2 delivery plan; checked first):
#   services/lifecycle/cmd/lifecycle/main.go   (HTTP API + reaper — Go, mandatory)
#   services/lifecycle/cmd/worker/main.go      (Temporal worker — Go, mandatory)
#   deploy/m2/Dockerfile.services
#   deploy/m2/executor-stub/Dockerfile   deploy/m2/executor-stub/entrypoint.sh
#   deploy/m2/compose.yaml
#   deploy/m2/tests/run-task.sh   deploy/m2/tests/mixed-runs.sh
#
# Abort condition (M2 exit gate): if V4 crash-recovery cannot be made to pass
# after remediation, STOP the POC sequence — R1 is load-bearing.
# Rollback (doc §5): docker compose down + manual reap of stray containers.
#
# FOR THE OPERATOR (junior dev) — read this before running:
#   1. M0 + M1 gates signed. Set M0_DONE=1 M1_DONE=1 to skip the live probes
#      when you know they're green.
#   2. `bash deploy/m2/install.sh --check-only`   # preview: no changes
#   3. `bash deploy/m2/install.sh`                # build 3 images -> deploy ->
#      gate tests V1..V5. V4 (crash recovery) is the POC's load-bearing test.
#   4. If V4 FAILs: fix per M2 install doc §6, re-run V4 then V5. If it
#      still cannot pass: STOP — that is the POC abort condition (owner
#      decision). Do not paper over it.
#   5. All V PASS -> run TP-M2 before starting M3.
#
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
M2="$REPO_ROOT/deploy/m2"
EVIDENCE="$M2/evidence"
LOG="$EVIDENCE/install-m2.log"
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

[ -f "$REPO_ROOT/deploy/m0/hosts.env" ] && { set -a; . "$REPO_ROOT/deploy/m0/hosts.env"; set +a; }
VM_XB="${VM_XB:-vm-xb}"
IMAGE_TAG="${IMAGE_TAG:-1.0.0}"

log()  { printf '[m2 %s] %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG"; }
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

# ---------------------------------------------------------------- prereqs
prereqs() {
  log "§0 — prereq checks (M0 + M1 gates)"
  [ "${M0_DONE:-0}" = "1" ] || {
    command -v temporal >/dev/null || die "temporal CLI missing on build host"
    temporal operator cluster health 2>/dev/null | grep -q HEALTHY || die "Temporal not healthy (M0)"
  }
  [ "${M1_DONE:-0}" = "1" ] || {
    if ! ssh "$VM_XB" "docker ps --format '{{.Names}}' | grep -q topic-bridge"; then
      die "topic-bridge not running on $VM_XB — run M1 install first (M1 gate: workflows startable)"
    fi
  }
  log "§0   container names assumed per the M2 doc: lifecycle + worker on $VM_XB"
  ok "prereqs"
}

# ---------------------------------------------------------------- assets
check_assets() {
  local missing=() f
  for f in \
    "$REPO_ROOT/services/lifecycle/cmd/lifecycle/main.go" \
    "$REPO_ROOT/services/lifecycle/cmd/worker/main.go" \
    "$M2/Dockerfile.services" \
    "$M2/executor-stub/Dockerfile" \
    "$M2/executor-stub/entrypoint.sh" \
    "$M2/compose.yaml" \
    "$M2/tests/run-task.sh" \
    "$M2/tests/mixed-runs.sh"
  do
    [ -e "$f" ] || missing+=("$f")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    if [ "$CHECK_ONLY" = "1" ]; then
      log "check-only: missing assets (not a failure in check-only):"
      printf '  %s\n' "${missing[@]}"
      exit 0
    fi
    log "missing assets (the M2 delivery plan produces these — Go, mandatory):"
    printf '  %s\n' "${missing[@]}"
    die "asset check failed — build the assets, then re-run"
  fi
  ok "assets present"
}

# ---------------------------------------------------------------- §1 build
build_and_load() {
  log "§1 — build 3 images (Go services — mandatory; stub runs zero agent logic)"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip build"; return 0; fi
  docker build -f "$M2/Dockerfile.services" \
    -t "poc/lifecycle:$IMAGE_TAG" -t "poc/worker:$IMAGE_TAG" \
    "$REPO_ROOT/services/lifecycle"
  docker build -f "$M2/executor-stub/Dockerfile" \
    -t "poc/executor-stub:$IMAGE_TAG" "$M2/executor-stub"
  for img in "poc/lifecycle:$IMAGE_TAG" "poc/worker:$IMAGE_TAG" "poc/executor-stub:$IMAGE_TAG"; do
    docker save "$img" | ssh "$VM_XB" "docker load"
    log "§1   loaded $img on $VM_XB"
  done
  ok "§1 build + load"
}

# ---------------------------------------------------------------- §2 deploy
deploy_lifecycle() {
  log "§2 — deploy lifecycle + worker on $VM_XB"
  confirm "deploy lifecycle + worker on $VM_XB (compose up -d)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  ssh "$VM_XB" "sudo mkdir -p /opt/poc/deploy/m2 && sudo chown -R \"$(id -un):$(id -gn)\" /opt/poc"
  rsync -az "$M2/compose.yaml" "$VM_XB:/opt/poc/deploy/m2/compose.yaml"
  # RABBITMQ_PASS feeds compose interpolation (${RABBITMQ_PASS} in compose.yaml);
  # sourced from the M0 rabbitmq .env — one password, one source of truth.
  local rmp; rmp="$(grep '^RABBITMQ_PASS=' "$REPO_ROOT/deploy/m0/rabbitmq/.env" | cut -d= -f2)"
  [ -n "$rmp" ] || die "RABBITMQ_PASS missing — run deploy/m0/install.sh §3 first"
  printf '%s\n' "RABBITMQ_PASS=$rmp" | ssh "$VM_XB" 'umask 077; cat > /opt/poc/deploy/m2/.env && chmod 600 /opt/poc/deploy/m2/.env'   # stdin + 600: secret not in argv/logs
  ssh "$VM_XB" "cd /opt/poc/deploy/m2 && docker compose up -d"
  # no-pull deploy gate: pinned image must exist locally before serving
  ssh "$VM_XB" "docker image inspect poc/executor-stub:$IMAGE_TAG >/dev/null" \
    || die "no-pull gate: pinned image poc/executor-stub:$IMAGE_TAG missing on $VM_XB (re-run §1)"
  # pre-deploy gate: lifecycle /health inside the container
  local i
  for i in $(seq 1 40); do
    if ssh "$VM_XB" "docker exec m2-worker-1 bash -c 'exec 3<>/dev/tcp/lifecycle/8090 && printf \"GET /health HTTP/1.0\r\nHost: lifecycle\r\n\r\n\" >&3 && head -1 <&3 | grep -q \" 200 \"'" >/dev/null 2>&1; then
      break
    fi
    sleep 3
    if [ "$i" = "40" ]; then die "lifecycle /health never came up (M2 §6: docker socket mount / DOCKER_HOST)"; fi
  done
  log "§2   worker registered check"
  ssh "$VM_XB" "temporal operator workflow-type list 2>/dev/null || true"
  ok "§2 lifecycle + worker deployed, no-pull gate green"
}

# ---------------------------------------------------------------- §3 gate tests
gate_tests() {
  log "§3 — verification suite V1..V5 (the M2 gate tests) -> $EVIDENCE/m2-tests.txt"
  local out="$EVIDENCE/m2-tests.txt"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip gate tests"; return 0; fi
  local R2="$M2/tests/run-task.sh"
  local v4pid t004 survivors
  local RUNID="$(date +%H%M%S)"
  {
    echo "# M2 gate tests — $(date -u)"
    # V1 clean path
    if bash "$R2" '{"agent_ref":"agent-roster/ephemeral-org/eph-fullstack-engineer","repo_target":"repos/ephemeral-poc-demo","requester":"principal","task_id":"t001-'"$RUNID"'","behavioral_spec":"Work package WP-T01: boot the ephemeral executor, write the start marker to /work/marker, and echo the config envelope (task_id, agent_ref, behavioral_spec) to stdout as the artifact. Acceptance criteria: marker exists; stdout contains all three config fields; process exits 0 within the TTL.","ttl_s":30}'; then
      echo "V1 clean: success + reaped : PASS"
    else
      echo "V1 clean : FAIL"
    fi
    # V2 failure path
    v2out="$(bash "$R2" '{"agent_ref":"agent-roster/ephemeral-org/eph-fullstack-engineer","repo_target":"repos/ephemeral-poc-demo","requester":"principal","task_id":"t002-'"$RUNID"'","behavioral_spec":"Work package WP-T02 (injected failure): same envelope as WP-T01, but the package is deliberately malformed - the executor must fail loudly. Acceptance criteria: non-zero exit; failure reason written to stdout before exit.","inject":"exit-1","ttl_s":30}')" || v2rc=$?
    if echo "$v2out" | grep -qiE '"status":"fail"'; then
      echo "V2 failure: surfaced + reaped : PASS"
    else
      echo "V2 failure : FAIL"
    fi
    # V3 timeout path
    v3out="$(bash "$R2" '{"agent_ref":"agent-roster/ephemeral-org/eph-fullstack-engineer","repo_target":"repos/ephemeral-poc-demo","requester":"principal","task_id":"t003-'"$RUNID"'","behavioral_spec":"Work package WP-T03 (bounded runtime): hold the executor alive for the full TTL with no artifact returned. Acceptance criteria: the lifecycle TTL backstop (poc.expires) fires first; the reaper reaps the container; the workflow surfaces timed-out.","inject":"hang","ttl_s":15}')" || v3rc=$?
    if echo "$v3out" | grep -qiE '"status":"timeout"'; then
      echo "V3 timeout: reaped at TTL + error surfaced : PASS"
    else
      echo "V3 timeout : FAIL"
    fi
    # V4 crash recovery — THE load-bearing test
    log "V4 — crash recovery: spawn t004 (hang, ttl 120), kill worker, reaper must get it"
    bash "$R2" '{"agent_ref":"agent-roster/ephemeral-org/eph-fullstack-engineer","repo_target":"repos/ephemeral-poc-demo","requester":"principal","task_id":"t004-'"$RUNID"'","behavioral_spec":"Work package WP-T04 (crash recovery): identical bounded-runtime behavior to WP-T03, with the added condition that the worker process dies mid-package. Acceptance criteria: the orphaned container is still reaped by the independent lifecycle reaper - zero survivors, regardless of worker availability.","inject":"hang","ttl_s":120}' &
    v4pid=$!
    sleep 5
    ssh "$VM_XB" "docker kill m2-worker-1"
    # FIX (run4 root cause): reaper reaps at poc.expires + REAPER_GRACE(30s) + up to
    # one interval; expiry is 120s after spawn, so the doc's bare "sleep 35" evaluated
    # ~80s BEFORE the reaper could act. Wait past expiry + grace + interval + margin.
    sleep 185
    t004="$(ssh "$VM_XB" "docker ps -a --filter label=poc.task=t004-${RUNID} -q || true")"
    ssh "$VM_XB" "docker start m2-worker-1"
    wait "$v4pid" || true
    if [ -z "$t004" ]; then
      echo "V4 crash: reaper got t004's container (zero survivors) : PASS"
    else
      echo "V4 crash : FAIL (survivor: $t004)"
    fi
    # V5 10 mixed runs, zero survivors
    if bash "$M2/tests/mixed-runs.sh" 10; then
      echo "V5 mixed-10: all events logged : PASS"
    else
      echo "V5 mixed-10 : FAIL"
    fi
    survivors="$(ssh "$VM_XB" "docker ps -a --filter label=poc.owner=lifecycle -q | wc -l")"
    echo "V5 zero-survivors: $survivors : $([ "$survivors" -eq 0 ] && echo PASS || echo FAIL)"
  } | tee "$out"
  if grep -q "FAIL" "$out"; then
    if grep -q "^V4 crash : FAIL" "$out"; then
      warn "V4 crash-recovery FAILED — ABORT CONDITION: remediate per M2 §6 (reaper query filter, TTL labels, chrony clock), re-run V4 then V5. If it still cannot pass: STOP the POC sequence (R1 is load-bearing) — owner decision."
    else
      warn "M2 gate tests: FAILURES above — remediate per M2 §6, re-run from the failed test."
    fi
    if [ "$NO_GATE" != "1" ]; then exit 1; fi
  else
    ok "M2 gate tests: V1..V5 PASS"
    log "Next: run TP-M2 — version 1/test procedures/M2-Lifecycle-Test-Procedure.md (file evidence in $EVIDENCE)."
  fi
}

# ---------------------------------------------------------------- §4 observability
observability_check() {
  log "§4 — observability spot-checks (M2.5)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  ssh "$VM_XB" "docker exec m2-worker-1 bash -c 'exec 3<>/dev/tcp/lifecycle/8090 && printf \"GET /report HTTP/1.0\r\nHost: lifecycle\r\n\r\n\" >&3 && cat <&3' 2>/dev/null | tail -n +2" \
    | tee "$EVIDENCE/m2-report-snapshot.txt" >/dev/null \
    || warn "GET /report not yet available (M2.5) — record, do not gate"
  ssh "$VM_XB" "ls /var/poc/tasks/ 2>/dev/null || true" | tee "$EVIDENCE/m2-task-logs-list.txt"
  ok "§4 observability evidence filed"
}

# ---------------------------------------------------------------- main
main() {
  log "=== M2 lifecycle install start (check-only=$CHECK_ONLY no-gate=$NO_GATE) ==="
  prereqs
  check_assets
  confirm "M2: build + deploy + gate-test lifecycle/worker/stub"
  build_and_load
  deploy_lifecycle
  gate_tests
  observability_check
  log "=== M2 install complete ==="
}
main
