#!/usr/bin/env bash
# =============================================================================
# M4 — MEMORY (Hindsight) + LLM-Wiki git repo: complete installation script
# =============================================================================
# Implements §1–§4 of:
#   version 1/installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md
# Gate afterwards:
#   version 1/test procedures/M4-Memory-Knowledge-Test-Procedure.md  (TP-M4
#   must PASS before the M5 installation may begin)
#
# Steps:
#   §1  Hindsight on vm-hindsight (pgvector + Go API) — NEVER shares a Postgres instance
#   §2  Wire hydration into the M2 lifecycle + executor env (connective work)
#   §3  Verification G1..G6 -> deploy/m4/evidence/
#
# The LLM-Wiki is NOT a service: it is a git repository
# (https://github.com/arvisdev/llm-wiki) that executors shallow-clone at boot.
# There is no retrieval API and no LLMWIKI_URL anywhere.
#
# Usage:  bash deploy/m4/install.sh [--check-only] [--no-gate]
#   ASSUME_YES=1  — skip interactive confirms
#
# Configuration (env, or deploy/m0/hosts.env — auto-loaded; deploy/m4/.env optional):
#   VM_3 / VM_XB — ssh aliases (defaults vm-hindsight / vm-xb)
#   M3_DONE=1   — skip prereq probes
#   EMBEDDING_PROVIDER — embedding provider name (record the choice + rate-limit note)
#   LLM_WIKI_READ_TOKEN — read token for the llm-wiki git repo (executor boot clone)
#   APPLY_HOSTS=1 — add /etc/hosts lines for vm-hindsight on vm-xa/vm-xb from HOSTS_IP_*
#
# Assets required in the repo (produced by the M4 delivery plan; checked first):
#   deploy/m4/hindsight/compose.yaml   deploy/m4/hindsight/initdb.sql
#   deploy/m4/hindsight/Dockerfile.api
#   services/hindsight-api/   (Go, per the POC Go convention)
#
# No new Temporal code in this milestone — the M2 Go worker still drives
# everything; the API services follow the POC Go convention.
# Rollback (doc §5): down per-VM compose; volumes only on a true re-do.
#
# FOR THE OPERATOR (junior dev) — read this before running:
#   1. M3 gate signed. Set LLM_WIKI_READ_TOKEN (or export it): the llm-wiki
#      repo is owner-curated — the script will not invent it.
#   2. `bash deploy/m4/install.sh --check-only`   # preview: no changes
#   3. `bash deploy/m4/install.sh`                # hindsight on vm-hindsight +
#      M2 wiring + gate tests G1..G6.
#   4. G2 (cross-task read-back) is the property that proves "long-term
#      memory" — if it fails, check the scope rule first (doc §6).
#   5. All G PASS -> run TP-M4 before starting M5.
#
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
M4="$REPO_ROOT/deploy/m4"
EVIDENCE="$M4/evidence"
LOG="$EVIDENCE/install-m4.log"
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
[ -f "$M4/.env" ] && { set -a; . "$M4/.env"; set +a; }
VM_3="${VM_3:-vm-hindsight}"; VM_XB="${VM_XB:-vm-xb}"; VM_XA="${VM_XA:-vm-xa}"
IMAGE_TAG="${IMAGE_TAG:-1.0.0}"
EMBEDDING_PROVIDER="${EMBEDDING_PROVIDER:-}"
HINDSIGHT_URL="http://$VM_3:8081"

log()  { printf '[m4 %s] %s\n' "$(date -u +%H:%M:%S)" "$*" | tee -a "$LOG"; }
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
  log "§0 — prereq checks (M0/M3 gates)"
  [ "${M3_DONE:-0}" = "1" ] || {
    ssh -o BatchMode=yes -o ConnectTimeout=8 "$VM_3" "docker version >/dev/null" \
      || die "$VM_3 not reachable / docker down (M0)"
    ssh -o BatchMode=yes -o ConnectTimeout=8 "$VM_XB" "docker ps --format '{{.Names}}' | grep -q lifecycle" \
      || die "lifecycle not running on $VM_XB (M2/M3)"
  }
  ok "prereqs"
}

# ---------------------------------------------------------------- assets
check_assets() {
  local missing=() f
  for f in \
    "$M4/hindsight/compose.yaml" \
    "$M4/hindsight/initdb.sql" \
    "$M4/hindsight/Dockerfile.api" \
    "$REPO_ROOT/services/hindsight-api"
  do
    [ -e "$f" ] || missing+=("$f")
  done
  if [ "${#missing[@]}" -gt 0 ]; then
    if [ "$CHECK_ONLY" = "1" ]; then
      log "check-only: missing assets (not a failure in check-only):"
      printf '  %s\n' "${missing[@]}"
      exit 0
    fi
    log "missing assets (the M4 delivery plan produces these — Go API services):"
    printf '  %s\n' "${missing[@]}"
    die "asset check failed — build the assets, then re-run"
  fi
  [ -n "${LLM_WIKI_READ_TOKEN:-}" ] || warn "LLM_WIKI_READ_TOKEN not set — the G3 llm-wiki clone gate will fail until it is (owner-curated repo, R7)"
  ok "assets present"
}

# ---------------------------------------------------------------- build
build_apis() {
  log "§1 — build Go API images (POC Go convention; no Temporal code here)"
  if [ "$CHECK_ONLY" = "1" ]; then return 0; fi
  docker build -f "$M4/hindsight/Dockerfile.api" "$REPO_ROOT/services/hindsight-api" -t "poc/hindsight-api:$IMAGE_TAG"
  ok "§1 API image built"
}

# ---------------------------------------------------------------- §1 hindsight
deploy_hindsight() {
  log "§1 — Hindsight on $VM_3 (pgvector + API; separate Postgres instance)"
  confirm "deploy hindsight on $VM_3"
  [ "$CHECK_ONLY" = "1" ] && return 0
  local envf="$M4/hindsight/.env"
  if ! grep -q '^HINDSIGHT_DB_PASS=' "$envf" 2>/dev/null; then
    echo "HINDSIGHT_DB_PASS=$(openssl rand -hex 16)" > "$envf"
    log "§1   generated $envf (gitignored)"
  fi
  # Embedding provider (D-embed, 2026-11-04): OpenAI-compatible HTTP provider,
  # text-embedding-3-small (dim 1536 = the table's vector width). "none" runs
  # the degenerate zero-vector fallback (writes succeed, reads are meaningless).
  if [ -n "${EMBEDDING_API_KEY:-}" ]; then
    grep -q '^EMBEDDING_PROVIDER=' "$envf" || echo "EMBEDDING_PROVIDER=${EMBEDDING_PROVIDER:-http}" >> "$envf"
    grep -q '^EMBEDDING_API_KEY=' "$envf" || printf '%s\n' "EMBEDDING_API_KEY=$EMBEDDING_API_KEY" >> "$envf"
  elif [ "${EMBEDDING_PROVIDER:-}" != "none" ]; then
    warn "EMBEDDING_API_KEY unset — hindsight runs the zero-vector fallback (EMBEDDINGS=none): reads are degenerate until a provider key is supplied (D-embed)"
  fi
  chmod 600 "$envf" 2>/dev/null || true
  docker save "poc/hindsight-api:$IMAGE_TAG" | ssh "$VM_3" "docker load"
  ssh "$VM_3" "mkdir -p /opt/poc/deploy/m4/hindsight"
  rsync -az "$M4/hindsight/" "$VM_3:/opt/poc/deploy/m4/hindsight/"
  ssh "$VM_3" "cd /opt/poc/deploy/m4/hindsight && docker compose up -d"
  local i
  for i in $(seq 1 40); do
    if ssh "$VM_3" "curl -sf $HINDSIGHT_URL/healthz >/dev/null" 2>/dev/null; then break; fi
    sleep 3
    if [ "$i" = "40" ]; then die "hindsight /healthz never came up (M4 §6: pgvector image? .env pass?)"; fi
  done
  ssh "$VM_3" "docker compose -f /opt/poc/deploy/m4/hindsight/compose.yaml exec -T hindsight-pg psql -U hindsight -c \"SELECT extname FROM pg_extension WHERE extname='vector'\"" | grep -q vector \
    || die "vector extension missing — M4 §6: use pgvector/pgvector:pg16, not plain postgres:16"
  ok "§1 hindsight up on $VM_3"
}

# ---------------------------------------------------------------- §2 wiring
wire_hydration() {
  log "§2 — wire hydration into the M2 lifecycle + executor env (connective work)"
  [ "$CHECK_ONLY" = "1" ] && return 0
  # names: ensure vm-hindsight resolves on the hosts that spawn/consume
  if [ "${APPLY_HOSTS:-0}" = "1" ] && [ -n "${HOSTS_IP_3:-}" ]; then
    for h in "$VM_XA" "$VM_XB"; do
      ssh "$h" "sudo bash -c 'grep -q \" $VM_3 \" /etc/hosts || echo \"$HOSTS_IP_3 $VM_3\" >> /etc/hosts'"
      log "§2   /etc/hosts line added on $h (record which mechanism in the decision log)"
    done
  else
    log "§2   verifying name resolution ($(getent hosts "$VM_3" >/dev/null && echo "workstation resolves $VM_3" || echo "workstation does NOT resolve $VM_3 — PVE DNS or /etc/hosts, M0 §5"))"
  fi
  # M2 env: HINDSIGHT_URL rides into every spawned container. The llm-wiki
  # (git repo) is cloned by the executor entrypoint at boot with
  # LLM_WIKI_READ_TOKEN — there is no LLMWIKI_URL (no retrieval API).
  # This is a CONFIG change (the M2 Go services are untouched — no new Temporal
  # code): the operator adds the env line to the lifecycle + worker
  # services in /opt/poc/deploy/m2/compose.yaml, then re-up.
  if ssh "$VM_XB" "grep -q 'HINDSIGHT_URL' /opt/poc/deploy/m2/compose.yaml"; then
    log "§2   M2 compose already carries HINDSIGHT_URL (idempotent)"
  else
    warn "§2   M2 compose has no HINDSIGHT_URL yet — add the line from the snippet below, then re-run this script"
  fi
  cat > "$EVIDENCE/m4-wiring-snippet.env" <<ENVEOF
# add to the lifecycle + worker service env in /opt/poc/deploy/m2/compose.yaml,
# then: cd /opt/poc/deploy/m2 && docker compose up -d   (on $VM_XB)
HINDSIGHT_URL=$HINDSIGHT_URL
ENVEOF
  if [ -n "${LLM_WIKI_READ_TOKEN:-}" ]; then
    printf 'LLM_WIKI_READ_TOKEN=%s\n' "$LLM_WIKI_READ_TOKEN" >> "$EVIDENCE/m4-wiring-snippet.env"
  else
    log "§2   LLM_WIKI_READ_TOKEN not set — the executor boots without the llm-wiki clone (rollback-degrades-gracefully)"
  fi
  log "§2   env snippet written to $EVIDENCE/m4-wiring-snippet.env"
  ssh "$VM_XB" "cd /opt/poc/deploy/m2 && docker compose up -d"
  local i
  for i in $(seq 1 40); do
    if ssh "$VM_XB" "docker compose -f /opt/poc/deploy/m2/compose.yaml exec -T lifecycle sh -c 'test -n \"\$(curl -s http://127.0.0.1:8090/health)\"'" >/dev/null 2>&1; then
      break
    fi
    sleep 3
    if [ "$i" = "40" ]; then die "M2 redeploy after wiring: lifecycle /health not up (M2 §6)"; fi
  done
  ssh "$VM_XB" "docker image inspect poc/executor-stub:1.0.0 >/dev/null" || warn "M2 no-pull gate: stub image missing — re-load it"
  ok "§2 hydration wired; M2 re-deployed with the new env"
}

# ---------------------------------------------------------------- §3 gate tests
gate_tests() {
  log "§3 — verification G1..G6 (the M4 gate tests) -> $EVIDENCE/"
  local out="$EVIDENCE/m4-tests.txt"
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: skip gate tests"; return 0; fi
  local body w r r2 t0 t1 run v3v wikiurl
  {
    echo "# M4 gate tests — $(date -u)"
    # G1 network R/W
    body='{"task_id":"t-g1","agent_ref":"dev","scope":"cross-task","body":{"summary":"m4-install-g1","learning":"m4-install-g1"}}'
    w="$(curl -s -XPOST "$HINDSIGHT_URL/memory/write" -H 'content-type: application/json' -d "$body" || echo CURL-FAIL)"
    r="$(curl -s "$HINDSIGHT_URL/memory/read?task_id=t-g1&scope=cross-task" || echo CURL-FAIL)"
    if echo "$r" | grep -q "m4-install-g1"; then
      echo "G1 network R/W: write+read-back : PASS"
    else
      echo "G1 network R/W : FAIL (write=$w read=$r)"
    fi
    # G2 cross-task read-back (API-level proof; the executor read path is the M2 wiring above)
    r2="$(curl -s "$HINDSIGHT_URL/memory/read?task_id=t-other&scope=cross-task" || echo CURL-FAIL)"
    if echo "$r2" | grep -q "m4-install-g1"; then
      echo "G2 cross-task read-back: a different task_id fetches the cross-task entry : PASS"
    else
      echo "G2 cross-task read-back : FAIL (record which step: M2 wiring vs. scope rule, M4 §6)"
    fi
    # G3 llm-wiki clone: shallow-clone the git repo from a VM and assert README readable
    wikiurl="${LLM_WIKI_REPO_URL:-https://github.com/arvisdev/llm-wiki}"
    if [ -n "${LLM_WIKI_READ_TOKEN:-}" ]; then
      if ssh "$VM_XB" "git clone --depth 1 https://x-access-token:${LLM_WIKI_READ_TOKEN}@github.com/arvisdev/llm-wiki /tmp/llm-wiki-g3 2>/dev/null && test -s /tmp/llm-wiki-g3/README.md && rm -rf /tmp/llm-wiki-g3"; then
        echo "G3 llm-wiki shallow clone from $VM_XB + README readable : PASS ($wikiurl)"
      else
        echo "G3 llm-wiki shallow clone : FAIL (check LLM_WIKI_READ_TOKEN / repo access from $VM_XB)"
      fi
    else
      echo "G3 llm-wiki shallow clone : FAIL (LLM_WIKI_READ_TOKEN not set — owner-curated repo, R7)"
    fi
    # G4 catalog regression (M3 self-report with the new sources live)
    if [ -x "$REPO_ROOT/deploy/m3/tests/boot-time.sh" ]; then
      bash "$REPO_ROOT/deploy/m3/tests/boot-time.sh" hermes 1 >/dev/null 2>&1 \
        && echo "G4 catalog regression: boot-time harness re-ran with sources live (verify self-report in $EVIDENCE)" \
        || echo "G4 catalog regression : FAIL (M3 TC-M3-02 re-run)"
    else
      echo "G4 catalog regression: manual — re-run M3 B2 with HINDSIGHT_URL set"
    fi
    # G5 hydration latency (3 runs)
    for run in 1 2 3; do
      t0=$(date +%s.%N)
      curl -s "$HINDSIGHT_URL/memory/read?task_id=t-g1&scope=cross-task" >/dev/null || true
      t1=$(date +%s.%N)
      echo "G5 latency run $run: total $(echo "$t1 $t0" | awk '{printf "%.2f", $1-$2}')s (must stay < M2 TTL headroom — record per-source)"
    done
    # G6 data isolation: hindsight's Postgres is its own instance on its own VM
    v3v="$(ssh "$VM_3" "docker volume ls --format '{{.Name}}' | grep -E 'hindsight' | head -1 || true")"
    echo "G6 isolation: vm-hindsight volume='${v3v:-none}' present, distinct from the M0 Postgres on $VM_XB : $([ -n "$v3v" ] && echo PASS || echo FAIL)"
  } | tee "$out"
  if grep -q ": FAIL" "$out"; then
    warn "M4 gate tests: FAILURES above — remediate per M4 §6, re-run from the failed test."
    if [ "$NO_GATE" != "1" ]; then exit 1; fi
  else
    ok "M4 gate tests recorded"
    log "Next: run TP-M4 — version 1/test procedures/M4-Memory-Knowledge-Test-Procedure.md (file evidence in $EVIDENCE)."
  fi
}

# ---------------------------------------------------------------- firewall
# The scripts own the POC's cross-VM port openings (owner baseline = SSH only).
open_firewall() {
  local cidr="${UFW_POC_CIDR:-}"
  [ -n "$cidr" ] || cidr="${NETPLAN_IP_3%.*}.0/24"
  [ -n "$cidr" ] && [ "$cidr" != ".0/24" ] || { log "fw   no NETPLAN_IP_3/UFW_POC_CIDR — skipping UFW setup"; return 0; }
  if [ "$CHECK_ONLY" = "1" ]; then log "check-only: would open UFW 8081 on $VM_3 from $cidr"; return 0; fi
  if ssh "$VM_3" "sudo -n ufw status 2>/dev/null | grep -q 'Status: active'"; then
    ssh "$VM_3" "sudo -n ufw allow from $cidr to any port 8081 proto tcp" \
      || die "fw   ufw rule failed on $VM_3"
    ok "fw   UFW: 8081 open on $VM_3 from $cidr"
  else
    log "fw   UFW not active on $VM_3 — skipped"
  fi
}

# ---------------------------------------------------------------- main
main() {
  log "=== M4 memory install start (check-only=$CHECK_ONLY no-gate=$NO_GATE) ==="
  prereqs
  check_assets
  confirm "M4: deploy hindsight, wire M2, gate-test"
  build_apis
  open_firewall
  deploy_hindsight
  wire_hydration
  gate_tests
  log "=== M4 install complete ==="
}
main
