#!/usr/bin/env bash
# =============================================================================
# M3 — EXECUTOR SPIKES (Hermes Ephemeral + Claude Code): complete install
# =============================================================================
# Implements §1–§5 of:
#   version 1/installation-deployment/M3-Executors-Installation-and-Deployment.md
# Gate afterwards:
#   version 1/test procedures/M3-Executors-Test-Procedure.md  (TP-M3 must PASS
#   before the M4 installation may begin)
#
# Steps:
#   §1  GitHub-side checks (catalog repo, project repo, App key, token dry-run)
#   §2  Build + load poc/hermes-ephemeral:1.0.0 + poc/claude-coder:1.0.0
#       (executors run their own runtimes — tooling, NOT Temporal code, so
#       outside the Go mandate; no M1/M2 Go code is touched here)
#   §2' Select the executor images in the M2 config (POC_IMAGE switch)
#   §3  Concurrency model decision (recorded, default: one-coder-per-task)
#   §4  Verification B1..B5 (+B6 if N-parallel) -> deploy/m3/evidence/
#   §5  Secret hygiene checks (env dumps, image history, trivy)
#
# Usage:  bash deploy/m3/install.sh [--check-only] [--no-gate]
#   ASSUME_YES=1  — skip interactive confirms
#
# Configuration (env, or deploy/m0/hosts.env — auto-loaded; deploy/m3/.env optional):
#   VM_XB  — ssh alias for vm-xb (default "vm-xb")
#   M2_DONE=1   — skip prereq probes
#   CATALOG_REPO / PROJECT_REPO — git URLs or owner/account-scoped names for gh checks
#   APP_KEY_PATH — default /opt/poc/secrets/app-key.pem on $VM_XB
#   N_PARALLEL=1 — choose the N-parallel concurrency model (default: one-coder)
#   PROJECT_LOCAL_DIR — local clone of the project repo on the build host (for B4 commit check)
#
# Assets required in the repo (produced by the M3 delivery plan; checked first):
#   deploy/m3/hermes-ephemeral/Dockerfile + entrypoint.sh
#   deploy/m3/claude-coder/Dockerfile     + entrypoint.sh
#   deploy/m3/issue-token.sh   (JWT-signed; curl + openssl, ~20 lines; runs on vm-xb)
#   deploy/m3/tests/boot-time.sh   deploy/m2/tests/run-task.sh
#
# Rollback (doc §6): rebuild + re-load; M2 keeps its stub pin until new tags
# pass, so a bad M3 image never takes down the lifecycle.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
M3="$REPO_ROOT/deploy/m3"
EVIDENCE="$M3/evidence"
LOG="$EVIDENCE/install-m3.log"
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
[ -f "$M3/.env" ] && { set -a; . "$M3/.env"; set +a; }
VM_XB="${VM_XB:-vm-xb}"
IMAGE_TAG="${IMAGE_TAG:-1.0.0}"
APP_KEY_PATH="${APP_KEY_PATH:-/opt/poc/secrets/app-key.pem}"
N_PARALLEL="${N_PARALLEL:-0}"

log()  { printf '[m3 %s] %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG"; }
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
  log "§0 — prereq checks (M2 gate)"
  [ "$M2_DONE" = "1" ] || {
    if ! ssh "$VM_XB" "docker ps --format '{{.Names}}' | grep -Eq '^(lifecycle|worker)$'"; then
      die "lifecycle/worker not running on $VM_XB — run M2 install first"
    fi
  }
  command -v gh >/dev/null || warn "gh CLI not found — GitHub-side checks (§1) will be printed, not executed"
  ok "prereqs"
}

# ---------------------------------------------------------------- assets
check_assets() {
  local missing=() f
  for f in \
    "$M3/hermes-ephemeral/Dockerfile" \
    "$M3/hermes-ephemeral/entrypoint.sh" \
    "$M3/claude-coder/Dockerfile" \
    "$M3/claude-coder/entrypoint.sh" \
    "$M3/issue-token.sh" \
    "$M3/tests/boot-time.sh"
  do
    [ -e "$f" ] || missing+=("$f")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    if [ "$CHECK_ONLY" = "1" ]; then
      log "check-only: missing assets (not a failure in check-only):"
      printf '  %s\n' "${missing[@]}"
      exit 0
    fi
    log "missing assets (the M3 delivery plan produces these):"
    printf '  %s\n' "${missing[@]}"
    die "asset check failed — build the assets, then re-run"
  fi
  ok "assets present"
}

# ---------------------------------------------------------------- §1 github-side
github_side() {
  log "§1 — GitHub-side checks (owner's workstation; do-once setup verified)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  if command -v gh >/dev/null; then
    for repo in "${CATALOG_REPO:-available-agent-list}" "${PROJECT_REPO:-poc-project}"; do
      if gh repo view "$repo" --json name >/dev/null 2>&1; then
        log "§1   repo $repo : present"
      else
        log "§1   repo $repo : MISSING (M3 §1a/1b — create it on the owner's account)"
      fi
    done
  fi
  # App private key on vm-xb (presence + perms only — never print it)
  if ssh "$VM_XB" "test -r $APP_KEY_PATH" ; then
    local perms; perms="$(ssh "$VM_XB" "stat -c '%a' $APP_KEY_PATH")"
    log "§1   app key present at $APP_KEY_PATH (perms $perms — must be 600)"
    [ "$perms" = "600" ] || warn "app key perms are $perms, expected 600 (M3 §1c)"
  else
    die "App private key missing on $VM_XB ($APP_KEY_PATH) — M3 §1c: create the GitHub App, save the PEM key, chmod 600. Never in git or any image."
  fi
  # token dry-run: issue + verify + revoke
  log "§1   token issuance dry-run (issue -> verify -> revoke)"
  if [ -x "$M3/issue-token.sh" ] || [ -f "$M3/issue-token.sh" ]; then
    local token
    token="$(bash "$M3/issue-token.sh" --dry-run)"
    [ -n "$token" ] || die "issue-token.sh produced no token (check key + installation_id, M3 §1c)"
    local code; code="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: token $token" https://api.github.com/user)"
    [ "$code" = "200" ] || die "issued token did not authenticate (HTTP $code)"
    bash "$M3/issue-token.sh" --revoke-last || true
    code="$(curl -s -o /dev/null -w '%{http_code}' -H "Authorization: token $token" https://api.github.com/user)"
    log "§1   token revocation check: HTTP $code (expected 401/403)"
    [ "$code" != "200" ] || warn "token still valid after revoke — GitHub App installation tokens expire at 1 h max (M3 §7)"
  else
    warn "issue-token.sh not executable / missing script body — dry-run skipped (owner must complete M3 §1c)"
  fi
  ok "§1 github-side"
}

# ---------------------------------------------------------------- §2 build
build_and_load() {
  log "§2 — build both executor images and load onto $VM_XB"
  confirm "build poc/hermes-ephemeral + poc/claude-coder (outside the Go mandate: these run their own tooling runtimes)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  docker build -f "$M3/hermes-ephemeral/Dockerfile" "$M3/hermes-ephemeral" -t "poc/hermes-ephemeral:$IMAGE_TAG"
  docker build -f "$M3/claude-coder/Dockerfile" "$M3/claude-coder" -t "poc/claude-coder:$IMAGE_TAG"
  docker save "poc/hermes-ephemeral:$IMAGE_TAG" "poc/claude-coder:$IMAGE_TAG" | ssh "$VM_XB" "docker load"
  ok "§2 executor images built + loaded"
}

select_images_in_m2() {
  log "§2' — select executor images in the M2 config (POC_IMAGE is a config switch, not a code change)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  if [ ! -f "$REPO_ROOT/deploy/m2/poc-images.env" ]; then
    cat > "$REPO_ROOT/deploy/m2/poc-images.env" <<ENVP
# M3 selection — per task type (default concurrency: one-coder-per-task)
POC_IMAGE_DEFAULT=poc/executor-stub:$IMAGE_TAG
ENVP
    log "§2'   wrote deploy/m2/poc-images.env (operator sets POC_IMAGE per task type: hermes / coder)"
  fi
  rsync -az "$REPO_ROOT/deploy/m2/compose.yaml" "$REPO_ROOT/deploy/m2/poc-images.env" "$VM_XB:/opt/poc/deploy/m2/" 2>/dev/null || true
  ssh "$VM_XB" "cd /opt/poc/deploy/m2 && docker compose restart lifecycle worker >/dev/null"
  ssh "$VM_XB" "docker image inspect poc/hermes-ephemeral:$IMAGE_TAG >/dev/null && docker image inspect poc/claude-coder:$IMAGE_TAG >/dev/null" \
    || die "M2 no-pull gate: executor images missing on $VM_XB after reload"
  ok "§2' M2 config switched to the selected images"
}

# ---------------------------------------------------------------- §3 concurrency
concurrency_decision() {
  log "§3 — concurrency model (R3)"
  if [ "$N_PARALLEL" = "1" ]; then
    log "§3   model: N-parallel coders via git worktrees (M2 config change: fan out N spawns)"
  else
    log "§3   model: DEFAULT one-coder-per-task (one container, one task/<id> branch, no shared checkout)"
  fi
  if [ "$CHECK_ONLY" != "1" ] && [ ! -f "$M3/concurrency-decision.md" ]; then
    {
      echo "# M3 concurrency model decision — $(date +%F)"
      echo "- Model: $([ "$N_PARALLEL" = "1" ] && echo "N-parallel + git worktrees" || echo "one-coder-per-task (default)")"
      echo "- Rationale: record here (R3: shared-checkout write conflicts vs. parallel throughput)"
      echo "- Impact: M2 config only — not a new image (M3 §3)"
    } > "$M3/concurrency-decision.md"
  fi
  ok "§3 concurrency model recorded"
}

# ---------------------------------------------------------------- §4 gate tests
gate_tests() {
  log "§4 — verification B1..B5$( [ "$N_PARALLEL" = "1" ] && echo B6 ) (the M3 gate tests) -> $EVIDENCE/"
  local out="$EVIDENCE/m3-tests.txt"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip gate tests"; return 0; fi
  {
    echo "# M3 gate tests — $(date -u)"
    # B1 boot-time x3
    bash "$M3/tests/boot-time.sh" hermes 3 2>&1 | tee "$EVIDENCE/boot-time-table.txt" \
      && echo "B1 boot-time: table recorded (record p50, set the acceptance bound, re-run to pass)"
    # B2 persona correctness
    if [ -f "$EVIDENCE/self-report.json" ] || [ -f "$M3/tests/self-report.json" ]; then
      echo "B2 persona: self-report present (verify role/skills/spec vs agents/dev-agent/ — record in $EVIDENCE)"
    else
      echo "B2 persona: RUN the self-report assertion manually (M3 §4 B2) and record"
    fi
    # B3 failure paths
    echo "B3 failure: operator runs (a) unreachable AGENT_REPO_URL and (b) malformed persona -> both exit 2 + reap (record in $EVIDENCE)"
    # B4 coder real work
    if [ -n "${PROJECT_LOCAL_DIR:-}" ] && [ -d "$PROJECT_LOCAL_DIR" ]; then
      local task_id="m3b-$(date +%H%M%S)"
      if bash "$REPO_ROOT/deploy/m2/tests/run-task.sh" "{\"task_id\":\"$task_id\",\"agent_ref\":\"coder\",\"brief\":\"add a /healthz handler\"}" >/dev/null 2>&1; then
        git -C "$PROJECT_LOCAL_DIR" fetch origin "task/$task_id" 2>/dev/null || true
        if git -C "$PROJECT_LOCAL_DIR" log --oneline "origin/task/$task_id" >/dev/null 2>&1; then
          echo "B4 coder: commit exists on origin/task/$task_id : PASS"
        else
          echo "B4 coder: task/$task_id not found on origin : FAIL (check M3 §7: contents:write on the project repo)"
        fi
      else
        echo "B4 coder: M2 run-task failed : FAIL"
      fi
    else
      echo "B4 coder: set PROJECT_LOCAL_DIR to a local clone of the project repo (manual per M3 §4 B4)"
    fi
    # B5 token lifecycle
    echo "B5 token: post-reap token must be 401 (re-run the §1 dry-run revoke check; record output)"
    # B6 only if N-parallel
    if [ "$N_PARALLEL" = "1" ]; then
      echo "B6 n-parallel: 2 concurrent coders -> 2 branches, both push, no lost work (record)"
    fi
  } | tee "$out"
  if grep -q ": FAIL" "$out"; then
    warn "M3 gate tests: FAILURES above — remediate per M3 §7, re-run from the failed test."
    if [ "$NO_GATE" != "1" ]; then exit 1; fi
  else
    ok "M3 gate tests recorded (complete the manual B2/B3/B5 lines in $out before signing TP-M3)"
    log "Next: run TP-M3 — version 1/test procedures/M3-Executors-Test-Procedure.md (file evidence in $EVIDENCE)."
  fi
}

# ---------------------------------------------------------------- §5 secret hygiene
secret_hygiene() {
  log "§5 — secret hygiene checks (R4, run before claiming done)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  local f1="$EVIDENCE/env-hermes.txt" f2="$EVIDENCE/env-coder.txt"
  docker run --rm "poc/hermes-ephemeral:$IMAGE_TAG" env > "$f1"
  docker run --rm "poc/claude-coder:$IMAGE_TAG" env > "$f2"
  if grep -Ei 'token|key|secret' "$f1" "$f2"; then
    die "secrets baked into an image env — M3 §5: secrets are injected at spawn, never baked (rebuild without them)"
  fi
  log "§5   image env dumps clean (secrets are injected at spawn)"
  for img in "poc/hermes-ephemeral:$IMAGE_TAG" "poc/claude-coder:$IMAGE_TAG"; do
    if docker history --no-trunc "$img" | grep -Eiq 'secret|token'; then
      die "a RUN layer wrote secrets into $img — M3 §5: rebuild without them"
    fi
  done
  log "§5   docker history: no secret-writing layers"
  if command -v trivy >/dev/null; then
    trivy image --exit-code 1 "poc/hermes-ephemeral:$IMAGE_TAG" "poc/claude-coder:$IMAGE_TAG" \
      && log "§5   trivy: secrets vuln class = 0" \
      || die "trivy found issues — remediate before claiming done"
  else
    warn "trivy not installed on the build host — install it (R4) and re-run this check"
  fi
  ok "§5 secret hygiene"
}

# ---------------------------------------------------------------- main
main() {
  log "=== M3 executor install start (check-only=$CHECK_ONLY no-gate=$NO_GATE) ==="
  prereqs
  check_assets
  confirm "M3: github-side + build + select + gate-test the executor spikes"
  github_side
  build_and_load
  select_images_in_m2
  concurrency_decision
  gate_tests
  secret_hygiene
  log "=== M3 install complete ==="
}
main
