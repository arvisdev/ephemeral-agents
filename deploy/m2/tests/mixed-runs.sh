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
  if bash "$R2" "$json"; then echo "run $i ($1): PASS"; else echo "run $i ($1): FAIL"; rc=1; fi
}

for _ in $(seq 1 "$succ");  do run success '{"ttl_s":30}'; done
for _ in $(seq 1 "$failn"); do run failure '{"inject":"exit-1","ttl_s":30}'; done
for _ in $(seq 1 "$timeo"); do run timeout  '{"inject":"hang","ttl_s":15}'; done

# zero survivors — the V5 acceptance condition
leftover="$(ssh "${VM_XB:-vm-xb}" "docker ps -a --filter label=poc.owner=lifecycle -q" 2>/dev/null | grep -c . || true)"
echo "survivors after $N mixed runs: $leftover (want 0)"
[ "${leftover:-0}" -eq 0 ] || rc=1
exit "$rc"
