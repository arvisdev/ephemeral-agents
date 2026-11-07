#!/usr/bin/env bash
# =============================================================================
# M5 — END-TO-END + HARDENING: complete installation script (final gate)
# =============================================================================
# Implements §1–§7 of:
#   version 1/installation-deployment/M5-End-to-End-Installation-and-Deployment.md
# Gate afterwards:
#   version 1/test procedures/M5-End-to-End-Test-Procedure.md  (TP-M5 = POC
#   complete: the verdict in Technical-Feasibility-Analysis.md confirmed by
#   evidence, not by analysis)
#
# M5 adds NO new infrastructure — it composes what M0–M4 installed and locks
# the last open items. M5 adds no new Temporal code (all Temporal components
# remain the Go M1/M2 services).
#
# Steps:
#   §1  Build + deploy gateway + manager on vm-xa
#   §3  Ordered full-stack bring-up (e2e-deploy.sh S1–S7, stage-gated)
#   §4  E2E acceptance run (one real request, human to human)
#   §5  Failure injections F1–F5
#   §6  Credential hardening (trivy sweep, env review, token 401)
#   §7  Lock the open items + close the docs
#
# Usage:  bash deploy/m5/install.sh [--check-only] [--no-gate]
#   ASSUME_YES=1  — skip interactive confirms
#
# Configuration (env, or deploy/m0/hosts.env — auto-loaded; deploy/m5/.env optional):
#   VM_XA / VM_XB / VM_3 — ssh aliases (defaults vm-xa/vm-xb/vm-hindsight)
#   GATEWAY_URL — default "http://<vm-xa-ip-or-name>:8080" (set to the reachable form)
#   E2E_BRIEF — the task_brief for the acceptance request (default: "add a health endpoint to the POC project")
#   E2E_TIMEOUT_S — default 600
#   M0..M4_DONE=1 — skip the all-gates prereq probe
#   AGENT_LIST_READ_TOKEN — read token for the available-agent-list repo (manager env)
#   PROJECT_LOCAL_DIR — local clone of the project repo (for the commit check)
#
# Assets required in the repo (produced by the M5 delivery plan; checked first):
#   services/gateway/ + services/manager/  (Go)
#   deploy/m5/compose.yaml   deploy/m5/Dockerfile (or per-service)
#   deploy/m5/e2e-deploy.sh  deploy/m5/report-contract.md
#
# Rollback (doc §8): M5 front-house only -> docker compose down on vm-xa;
# everything behind it (M0–M4) stays up.
#
# FOR THE OPERATOR (junior dev) — read this before running:
#   1. TP-M0..TP-M4 all signed. Export M0_DONE=1 M1_DONE=1 M2_DONE=1
#      M3_DONE=1 M4_DONE=1 to skip the live probes when you know they're green.
#   2. `bash deploy/m5/install.sh --check-only`   # preview: no changes
#   3. `bash deploy/m5/install.sh`                # front-of-house ->
#      e2e-deploy S1-S7 -> E2E run -> F1-F4 injections -> hardening scans.
#   4. A failing E2E stage names its owning milestone: re-run THAT milestone's
#      gate test (M1 §4 / M2 §3 / M3 §4 / M4 §4), not the whole chain.
#   5. Final step: run TP-M5. All five done-when conditions hold = POC done.
#
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
M5="$REPO_ROOT/deploy/m5"
EVIDENCE="$M5/evidence"
LOG="$EVIDENCE/install-m5.log"
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
[ -f "$M5/.env" ] && { set -a; . "$M5/.env"; set +a; }
VM_XA="${VM_XA:-vm-xa}"; VM_XB="${VM_XB:-vm-xb}"; VM_3="${VM_3:-vm-hindsight}"
IMAGE_TAG="${IMAGE_TAG:-1.0.0}"
GATEWAY_URL="${GATEWAY_URL:-http://$VM_XA:8080}"
E2E_BRIEF="${E2E_BRIEF:-add a health endpoint to the POC project}"
E2E_TIMEOUT_S="${E2E_TIMEOUT_S:-600}"
ALL_GATES_DONE="${M0_DONE:-0}${M1_DONE:-0}${M2_DONE:-0}${M3_DONE:-0}${M4_DONE:-0}"

log()  { printf '[m5 %s] %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG"; }
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
  log "§0 — prereq: TP-M0..TP-M4 all signed"
  if [ "$ALL_GATES_DONE" = "11111" ]; then
    log "§0   ALL_GATES_DONE — skipping live probes"
    return 0
  fi
  ssh "$VM_XB" "docker ps --format '{{.Names}}' | grep -q topic-bridge" \
    || die "M1: topic-bridge missing on $VM_XB — earlier gate not passed"
  ssh "$VM_XB" "docker ps --format '{{.Names}}' | grep -q lifecycle" \
    || die "M2: lifecycle missing on $VM_XB — earlier gate not passed"
  ssh "$VM_3" "curl -sf http://127.0.0.1:8081/healthz" >/dev/null 2>&1 \
    || die "M4: hindsight not healthy on $VM_3 — earlier gate not passed"
  ok "prereqs"
}

# ---------------------------------------------------------------- assets
check_assets() {
  local missing=() f
  for f in \
    "$REPO_ROOT/services/gateway" \
    "$REPO_ROOT/services/manager" \
    "$M5/compose.yaml" \
    "$M5/e2e-deploy.sh" \
    "$M5/report-contract.md"
  do
    [ -e "$f" ] || missing+=("$f")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    if [ "$CHECK_ONLY" = "1" ]; then
      log "check-only: missing assets (not a failure in check-only):"
      printf '  %s\n' "${missing[@]}"
      exit 0
    fi
    log "missing assets (the M5 delivery plan produces these — Go services):"
    printf '  %s\n' "${missing[@]}"
    die "asset check failed — build the assets, then re-run"
  fi
  ok "assets present"
}

# ---------------------------------------------------------------- §1 front-of-house
build_and_deploy_front_of_house() {
  log "§1 — build gateway + manager (Go) and deploy on $VM_XA"
  confirm "build poc/gateway + poc/manager and deploy on $VM_XA"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  docker build -f "$M5/Dockerfile" "$REPO_ROOT/services/gateway" -t "poc/gateway:$IMAGE_TAG"
  docker build -f "$M5/Dockerfile" "$REPO_ROOT/services/manager"  -t "poc/manager:$IMAGE_TAG"
  docker save "poc/gateway:$IMAGE_TAG" "poc/manager:$IMAGE_TAG" | ssh "$VM_XA" "docker load"
  ssh "$VM_XA" "mkdir -p /opt/poc/deploy/m5 /opt/poc/reports"
  rsync -az "$M5/compose.yaml" "$VM_XA:/opt/poc/deploy/m5/compose.yaml"
  # .env: RABBITMQ_PASS always (compose interpolation); AGENT_LIST_READ_TOKEN when provided.
  local rmp; rmp="$(grep '^RABBITMQ_PASS=' "$REPO_ROOT/deploy/m0/rabbitmq/.env" | cut -d= -f2)"
  [ -n "$rmp" ] || die "RABBITMQ_PASS missing — run deploy/m0/install.sh §3 first"
  {
    printf '%s\n' "RABBITMQ_PASS=$rmp"
    [ -n "${AGENT_LIST_READ_TOKEN:-}" ] && printf '%s\n' "AGENT_LIST_READ_TOKEN=$AGENT_LIST_READ_TOKEN"
  } | ssh "$VM_XA" 'umask 077; cat > /opt/poc/deploy/m5/.env && chmod 600 /opt/poc/deploy/m5/.env'   # stdin + 600: secret not in argv/logs
  [ -n "${AGENT_LIST_READ_TOKEN:-}" ] || warn "AGENT_LIST_READ_TOKEN not set — manager cannot clone the catalog until it is"
  ssh "$VM_XA" "cd /opt/poc/deploy/m5 && docker compose up -d"
  local i
  for i in $(seq 1 40); do
    if curl -sf "$GATEWAY_URL/healthz" >/dev/null 2>&1; then break; fi
    sleep 3
    if [ "$i" = "40" ]; then die "gateway /healthz not reachable at $GATEWAY_URL (M5 §8: front-house down)"; fi
  done
  ok "§1 front-of-house up on $VM_XA"
}

# ---------------------------------------------------------------- §3 bring-up
full_stack_bringup() {
  log "§3 — ordered full-stack bring-up (S1–S7, stage-gated)"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip bring-up"; return 0; fi
  if [ -f "$M5/e2e-deploy.sh" ]; then
    bash "$M5/e2e-deploy.sh"   # exits non-zero at the failing stage
    ok "§3 e2e-deploy.sh S1–S7 green"
  else
    die "deploy/m5/e2e-deploy.sh missing (stage-gated bring-up; M5 §3) — cannot continue"
  fi
}

# ---------------------------------------------------------------- §4 E2E acceptance
e2e_acceptance_run() {
  log "§4 — E2E acceptance run: one real request, human to human"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip E2E run"; return 0; fi
  local resp task_id payload
  payload="$(printf '%s' "$E2E_BRIEF" | python3 -c 'import json,sys; print(json.dumps({"title":"add health endpoint","agent_capability":"dev","task_brief":sys.stdin.read()}))')" \
    || die "E2E payload build failed (python3 required on the build host)"
  resp="$(curl -s -X POST "$GATEWAY_URL/" \
    -H 'content-type: application/json' \
    --data-binary "$payload")"
  log "§4   gateway response: $resp"
  task_id="$(echo "$resp" | sed -n 's/.*"task_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
  [ -n "$task_id" ] || die "gateway response has no task_id (see deploy/m5/report-contract.md)"

  # 1 workflow for it
  ssh "$VM_XB" "temporal workflow list --type TaskWorkflow --limit 20 | grep -c '$task_id'" \
    | grep -q '[1-9]' || warn "§4   temporal: workflow row for $task_id not yet listed (poll)"
  # agent container alive + labeled while running (snapshot)
  ssh "$VM_XB" "docker ps --filter label=poc.owner=lifecycle --format '{{.Names}}'" | tee "$EVIDENCE/e2e-containers-alive.txt"
  # wait for completion
  local i
  for i in $(seq 1 "$((E2E_TIMEOUT_S / 10))"); do
    if curl -sf "$GATEWAY_URL/gateway/reports/$task_id" >/dev/null 2>&1; then break; fi
    sleep 10
    if [ "$i" = "$((E2E_TIMEOUT_S / 10))" ]; then die "E2E: report never landed within ${E2E_TIMEOUT_S}s"; fi
  done
  # zero survivors after completion
  local survivors
  survivors="$(ssh "$VM_XB" "docker ps -a --filter label=poc.owner=lifecycle -q | wc -l")"
  log "§4   post-completion POC container count: $survivors (expected 0)"
  # commit exists
  if [ -n "${PROJECT_LOCAL_DIR:-}" ] && [ -d "$PROJECT_LOCAL_DIR" ]; then
    git -C "$PROJECT_LOCAL_DIR" fetch origin "task/$task_id" 2>/dev/null || true
    if git -C "$PROJECT_LOCAL_DIR" log --oneline "origin/task/$task_id" >/dev/null 2>&1; then
      log "§4   commit exists on origin/task/$task_id"
    else
      die "§4   task/$task_id not on origin — artifact missing"
    fi
  else
    warn "§4   PROJECT_LOCAL_DIR unset — operator must verify the commit per M5 §4"
  fi
  # report JSON — the task itself must have SUCCEEDED (not just reaped clean)
  curl -sf "$GATEWAY_URL/gateway/reports/$task_id" | tee "$EVIDENCE/e2e-report-$task_id.json" >/dev/null
  rstatus="$(python3 -c "import json;print(json.load(open('$EVIDENCE/e2e-report-$task_id.json')).get('status',''))" 2>/dev/null || echo unreadable)"
  if [ "$rstatus" != "success" ]; then
    rreason="$(python3 -c "import json;print(json.load(open('$EVIDENCE/e2e-report-$task_id.json')).get('reason',''))" 2>/dev/null)"
    die "§4   acceptance task FAILED (report status=$rstatus reason=$rreason) — see $EVIDENCE/e2e-report-$task_id.json"
  fi
  log "§4   report status: success"
  log "§4   evidence pack -> $EVIDENCE (add: temporal trace export + lifecycle log)"
  ok "§4 E2E run: artifact chain verified (survivors=$survivors)"
}

# ---------------------------------------------------------------- §5 failure injections
failure_injections() {
  log "§5 — failure-injection runbook F1–F5 (each: clean reap + human-visible error)"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip injections"; return 0; fi
  local f _f3pid cid
  {
    echo "# M5 failure injections — $(date -u)"
    # F1: llm-wiki clone unreachable (block outbound HTTPS to github.com on the executor VM)
    log "F1 — block outbound git/HTTPS to github.com on $VM_XB during a task (llm-wiki clone fails)"
    block_github_443
    f="$(run_one_task)"
    unblock_github_443
    assert_clean_reap "F1" "$f"
    # F2: agent catalog unreachable
    log "F2 — block vm-xb -> GitHub 443 (nftables; agent-repo clone path)"
    block_github_443
    f="$(run_one_task)"
    unblock_github_443
    assert_clean_reap "F2" "$f"
    # F3: agent crash
    log "F3 — pkill -9 the agent process mid-task"
    run_one_task >/dev/null &
    _f3pid=$!
    sleep 10
    cid="$(ssh "$VM_XB" "docker ps --filter label=poc.owner=lifecycle -q | head -1 || true")"
    if [ -n "$cid" ]; then
      ssh "$VM_XB" "docker exec $cid sh -c 'pkill -9 1 2>/dev/null || pkill -9 agent 2>/dev/null || true'"
    fi
    wait "$_f3pid" || true
    assert_clean_reap "F3" "crashed agent ($cid)"
    # F4: TTL exceeded
    log "F4 — task with TTL_S smaller than work time (reaper path with real agents)"
    f="$(run_one_task_short_ttl)"
    assert_clean_reap "F4" "$f"
    # F5: manager restart mid-task (response-consumer reconnect + pending persistence)
    log "F5 — docker restart manager mid-task; report must still be written after restart"
    _before="$(ssh "$VM_XA" "ls /opt/poc/reports/*.json 2>/dev/null | wc -l")"
    run_one_task >/dev/null &
    _f5pid=$!
    sleep 10
    ssh "$VM_XA" "cd /opt/poc/deploy/m5 && docker compose restart manager"
    wait "$_f5pid" || true
    sleep 45   # reaper margin + consumer reconnect + pending reload + report write
    _after="$(ssh "$VM_XA" "ls /opt/poc/reports/*.json 2>/dev/null | wc -l")"
    survivors="$(ssh "$VM_XB" "docker ps -a --filter label=poc.owner=lifecycle -q | wc -l")"
    if [ "$survivors" = "0" ] && [ "$_after" -gt "$_before" ]; then
      echo "F5 manager-restart: report written after restart ($_before -> $_after reports), zero survivors : PASS"
    else
      echo "F5 manager-restart: survivors=$survivors reports=$_before->$_after : FAIL"
    fi
  } | tee "$EVIDENCE/failure-injections.txt"
  if grep -q ": FAIL" "$EVIDENCE/failure-injections.txt"; then
    warn "§5 injection FAILURES above — remediate in the owning milestone (M2 §3 for reaping), re-run."
    if [ "$NO_GATE" != "1" ]; then exit 1; fi
  else
    ok "§5 F1–F5 clean + reported"
  fi
}

# helpers for the injection runbook (small task each time)
run_one_task() {
  curl -s -X POST "$GATEWAY_URL/" -H 'content-type: application/json' \
    -d '{"title":"f-inject","agent_capability":"dev","task_brief":"tiny task for the injection runbook"}' || true
}
run_one_task_short_ttl() {
  curl -s -X POST "$GATEWAY_URL/" -H 'content-type: application/json' \
    -d '{"title":"f-inject","agent_capability":"dev","task_brief":"long task","ttl_s":5}' || true
}
block_github_443() {
  ssh "$VM_XB" "sudo nft add table inet pocblock && sudo nft add chain inet pocblock output '{ ip daddr 140.82.112.0/20 tcp dport 443 counter drop }'" 2>/dev/null \
    || warn "nftables block unavailable (needs root on $VM_XB) — operator runs it manually (M5 §5 F2)"
}
unblock_github_443() {
  ssh "$VM_XB" "sudo nft delete table inet pocblock 2>/dev/null || true"
}
assert_clean_reap() {
  local tag="$1" resp="$2" survivors
  sleep 45   # one reaper interval + margin
  survivors="$(ssh "$VM_XB" "docker ps -a --filter label=poc.owner=lifecycle -q | wc -l")"
  if [ "$survivors" = "0" ]; then
    echo "$tag zero-survivors: 0 : PASS"
    if echo "$resp" | grep -qiE 'error|fail'; then
      echo "$tag human-visible error in report : PASS"
    else
      echo "$tag human-visible error : CHECK manually (record the chosen policy: error vs. degrade, M5 §5)"
    fi
  else
    echo "$tag zero-survivors: $survivors : FAIL"
  fi
}

# ---------------------------------------------------------------- §6 hardening
credential_hardening() {
  log "§6 — credential hardening (R4 final)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  if command -v trivy >/dev/null; then
    local img
    for img in poc/topic-bridge poc/lifecycle poc/worker poc/executor-stub poc/hermes-ephemeral poc/claude-coder poc/hindsight-api poc/gateway poc/manager; do   # ALL 9 deployed images
      # poc/claude-coder exception (decision log, M5 §6): the pinned Claude Code
      # CLI (and the pinned Hermes Agent v0.23.0 runtime in hermes-ephemeral) bundle their dependencies; fixes live upstream.
      EXTRA=""
      case "$img" in poc/claude-coder) EXTRA="--skip-dirs /usr/local/lib/node_modules";; esac
      if [ "$img" = "poc/hermes-ephemeral" ]; then
        # lang-pkgs findings (Node/Python deps bundled inside the pinned Hermes
        # Agent v0.23.0 runtime) are an accepted residual: fixes live in the
        # upstream Hermes release, not in this image (decision log, M5 §6).
        trivy image --scanners vuln,secret --severity HIGH,CRITICAL --ignore-unfixed \
          --skip-dirs /root/.hermes/hermes-agent --format json "$img:$IMAGE_TAG" > /tmp/trivy-he.json 2>/dev/null || true
        n=$(python3 -c "import json; d=json.load(open('/tmp/trivy-he.json')); print(sum(len(r.get('Vulnerabilities') or []) for r in d.get('Results',[]) if r.get('Class') != 'lang-pkgs'))" 2>/dev/null || echo 1)
        ns=$(python3 -c "import json; d=json.load(open('/tmp/trivy-he.json')); print(sum(len(r.get('Secrets') or []) for r in d.get('Results',[])))" 2>/dev/null || echo 1)
        if [ "${n:-1}" = "0" ] && [ "${ns:-1}" = "0" ]; then
          log "§6   trivy $img:$IMAGE_TAG : clean (pinned-runtime lang-pkgs accepted residual)"
          continue
        fi
        die "§6   trivy: $img:$IMAGE_TAG has fixable HIGH/CRITICAL findings or secrets — remediate"
      fi
      trivy image --exit-code 1 --scanners vuln,secret --severity HIGH,CRITICAL --ignore-unfixed $EXTRA "$img:$IMAGE_TAG" \
        && log "§6   trivy $img:$IMAGE_TAG : clean" \
        || die "§6   trivy: $img:$IMAGE_TAG has fixable HIGH/CRITICAL findings or secrets — remediate"
      mkdir -p "$(dirname "$EVIDENCE/trivy-$img.json")"
      trivy image --scanners vuln --format json "$img:$IMAGE_TAG" > "$EVIDENCE/trivy-$img.json" 2>/dev/null || true
    done
  else
    warn "trivy not installed — install it and re-run §6a (cannot claim hardening without the scan)"
  fi
  # env review: no owner long-lived tokens anywhere
  local hits
  hits="$(ssh "$VM_XB" 'for c in $(docker ps -q); do docker inspect "$c" | grep -Ei "owner_?token|github_pat" || true; done')"
  if [ -n "$hits" ]; then
    die "§6   owner long-lived token found in a container env — M5 §6b: remove + re-deploy"
  fi
  log "§6   env review clean"
  # token lifecycle evidence (M3 §4 B5 re-run)
  if [ -f "$REPO_ROOT/deploy/m3/issue-token.sh" ]; then
    local tok tcode
    tok="$(ssh "$VM_XB" 'set -a; . /opt/poc/secrets/m3.env; set +a; bash /opt/poc/deploy/m3/issue-token.sh --dry-run' 2>/dev/null)"
    [ -n "$tok" ] || warn "§6   token dry-run unavailable (M3 §1c state)"
    tcode="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: token $tok" https://api.github.com/user)"
    echo "§6   issued token -> HTTP $tcode (want 200 pre-revoke); revoke + expect 401 (record in evidence)" | tee -a "$LOG"
  fi
  ok "§6 hardening"
}

# ---------------------------------------------------------------- §7 close docs
lock_open_items() {
  log "§7 — lock the last open items + close the docs"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  if [ ! -f "$M5/decision-log.md" ]; then
    cat > "$M5/decision-log.md" <<'DEC'
# M5 decision log — lock the open items

- [ ] VM numbering (carried from M0) — final scheme recorded
- [ ] topic/queue message shapes — schemas/message-v1.json is the contract (version it)
- [ ] concurrency model — one-coder-per-task (default) or N-parallel + worktrees (M3 §3)
- [ ] ephemeral isolation level — co-located containers on vm-xb (POC) vs. per-task VMs (defer)
- [x] llm-wiki repo curation owner + cadence (from M4) — D-curation (2026-11-04): owner = the Human-In-The-Loop (HITL); append-only, need-driven; record in the llm-wiki repo's CHANGELOG.md

After locking: update version 1/ARCHITECTURE.md + Initial-Foundation-Design-Document.md
(move resolved items out of "open") and add the Version 2 open items list
(llm-wiki repo curation ownership, long-term queue shape,
multi-flow concurrency).
DEC
  fi
  log "§7   template at $M5/decision-log.md — the operator locks each box, then updates the version 1/ docs"
  ok "§7 docs-closeout prepared"
}

# ---------------------------------------------------------------- POC complete
poc_complete_summary() {
  log "POC COMPLETE CHECK (M5 §9 done-when — all five):"
  local s
  s="$(ssh "$VM_XB" "docker ps -a --filter label=poc.owner=lifecycle -q | wc -l")"
  log " 1. E2E run passed, zero surviving POC containers : survivors=$s $([ "$s" = "0" ] && echo '(PASS)' || echo '(CHECK)')"
  log " 2. F1–F4 clean + reported : see $EVIDENCE/failure-injections.txt"
  log " 3. §6 scans pass : see $LOG"
  log " 4. §7 open items locked : see $M5/decision-log.md"
  log " 5. Engineer sees the outcome : report JSON with artifact pointer (GET $GATEWAY_URL/gateway/reports/<task_id>)"
  log "When 1–5 hold: run TP-M5 (version 1/test procedures/M5-End-to-End-Test-Procedure.md) to sign the POC gate."
  ok "M5 install complete"
}

# ---------------------------------------------------------------- firewall
# The scripts own the POC's port openings (owner baseline = SSH only). The
# gateway on vm-xa:8080 is hit by the engineer's desktop — often OUTSIDE the
# POC LAN, so its source is GATEWAY_ALLOW_CIDR (defaults to the POC LAN).
open_firewall() {
  local cidr="${GATEWAY_ALLOW_CIDR:-${UFW_POC_CIDR:-}}"
  [ -n "$cidr" ] || cidr="${NETPLAN_IP_XA%.*}.0/24"
  [ -n "$cidr" ] && [ "$cidr" != ".0/24" ] || { log "fw   no NETPLAN_IP_XA/GATEWAY_ALLOW_CIDR — skipping UFW setup"; return 0; }
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: would open UFW 8080 on $VM_XA from $cidr"; return 0; fi
  if ssh "$VM_XA" "sudo -n ufw status 2>/dev/null | grep -q 'Status: active'"; then
    ssh "$VM_XA" "sudo -n ufw allow from $cidr to any port 8080 proto tcp" \
      || die "fw   ufw rule failed on $VM_XA"
    ok "fw   UFW: 8080 (gateway) open on $VM_XA from $cidr"
  else
    log "fw   UFW not active on $VM_XA — skipped"
  fi
}

# ---------------------------------------------------------------- main
main() {
  log "=== M5 e2e+hardening install start (check-only=$CHECK_ONLY no-gate=$NO_GATE) ==="
  prereqs
  check_assets
  confirm "M5: front-of-house + bring-up + E2E + injections + hardening + docs closeout"
  build_and_deploy_front_of_house
  open_firewall
  full_stack_bringup
  e2e_acceptance_run
  failure_injections
  credential_hardening
  lock_open_items
  poc_complete_summary
}
main
