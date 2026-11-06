#!/usr/bin/env bash
# =============================================================================
# M2 test driver — V5: N mixed runs, zero survivors
# =============================================================================
# Implements M2 install doc §3 V5 / delivery plan WP-2.4:
#   "10 mixed runs (5 success, 3 injected failure, 2 timeout) → zero surviving
#    POC containers afterwards."
#
# Usage: mixed-runs.sh [N]        (default 10)
# Mix per 10: 5 clean, 3 exit-1, 2 hang+short-TTL (scaled proportionally).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
R2="$SCRIPT_DIR/run-task.sh"
N="${1:-10}"
[ "$N" -ge 1 ] 2>/dev/null || { echo "usage: mixed-runs.sh [N]" >&2; exit 64; }

succ=$(( N * 5 / 10 )); [ "$succ" -ge 1 ] || succ=1
failn=$(( N * 3 / 10 ))
timeo=$(( N - succ - failn )); [ "$timeo" -ge 0 ] || timeo=0

echo "# mixed-runs: $N runs ($succ success / $failn failure / $timeo timeout)"
rc=0
i=0
run() { # run <kind> <json>
  i=$((i+1)); local id="mix-$(date +%s)-$i"
  local json
  json="$(printf '%s' "$2" | python3 -c "import json,sys;d=json.load(sys.stdin);d['task_id']='$id';print(json.dumps(d))")"
  echo "-- run $i/$N ($1): $id"
  # FIX (run7): expectation depends on the injected kind — a failure run is
  # CORRECT when run-task exits 1 (status fail); timeout likewise.
  local out expect_rc
  out="$(bash "$R2" "$json")" && expect_rc=0 || expect_rc=$?
  echo "$out"
  case "$1" in
    success) if [ "$expect_rc" -eq 0 ] && grep -q '"status":"success"' <<<"$out"; then echo "run $i ($1): PASS"; else echo "run $i ($1): FAIL"; rc=1; fi ;;
    failure) if [ "$expect_rc" -eq 1 ] && grep -q '"status":"fail"' <<<"$out"; then echo "run $i ($1): PASS"; else echo "run $i ($1): FAIL"; rc=1; fi ;;
    timeout) if [ "$expect_rc" -eq 1 ] && grep -q '"status":"timeout"' <<<"$out"; then echo "run $i ($1): PASS"; else echo "run $i ($1): FAIL"; rc=1; fi ;;
  esac
}

# payloads: behavioral_spec per deploy/m2/tests/test-work-packages.md (WP-T01/02/03);
# required config fields per schemas/message-v1.json (agent_ref/repo_target/requester)
SPEC1="Work package WP-T01: boot the ephemeral executor, write the start marker to /work/marker, and echo the config envelope (task_id, agent_ref, behavioral_spec) to stdout as the artifact. Acceptance criteria: marker exists; stdout contains all three config fields; process exits 0 within the TTL."
SPEC2="Work package WP-T02 (injected failure): same envelope as WP-T01, but the package is deliberately malformed - the executor must fail loudly. Acceptance criteria: non-zero exit; failure reason written to stdout before exit."
SPEC3="Work package WP-T03 (bounded runtime): hold the executor alive for the full TTL with no artifact returned. Acceptance criteria: the lifecycle TTL backstop (poc.expires) fires first; the reaper reaps the container; the workflow surfaces timed-out."
base_payload() { local spec; case "$1" in 1) spec="$SPEC1";; 2) spec="$SPEC2";; 3) spec="$SPEC3";; esac; printf '{"agent_ref":"agent-roster/ephemeral-org/eph-fullstack-engineer","repo_target":"repos/ephemeral-poc-demo","requester":"principal","behavioral_spec":"%s","ttl_s":%s,"inject":"%s"}' "$spec" "$2" "$3"; }

for _ in $(seq 1 "$succ");  do run success "$(base_payload 1 30 '')"; done
for _ in $(seq 1 "$failn"); do run failure "$(base_payload 2 30 'exit-1')"; done
for _ in $(seq 1 "$timeo"); do run timeout  "$(base_payload 3 15 'hang')"; done

# zero survivors — the V5 acceptance condition
leftover="$(ssh "${VM_XB:-vm-xb}" "docker ps -a --filter label=poc.owner=lifecycle -q" 2>/dev/null | grep -c . || true)"
echo "survivors after $N mixed runs: $leftover (want 0)"
[ "${leftover:-0}" -eq 0 ] || rc=1
exit "$rc"
