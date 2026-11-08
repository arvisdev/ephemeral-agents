#!/usr/bin/env bash
# =============================================================================
# M3 gate test B1 — executor boot-time measurement
# =============================================================================
# M3 install doc §4: "B1 boot-time (3 runs): spawn via M2 with a stub task;
# time: container start -> self-report.json ready.  Record p50; decide the
# acceptance bound, then re-run to pass."
#
# Usage: boot-time.sh <executor> <runs>      e.g. boot-time.sh hermes 3
# Env: VM_XB (default vm-xb), RUN_TASK (default deploy/m2/tests/run-task.sh)
#
# Output: a markdown table (per-run seconds + p50) on stdout — install.sh tees
# it to deploy/m3/evidence/boot-time-table.txt.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
RUN_TASK="${RUN_TASK:-$REPO_ROOT/deploy/m2/tests/run-task.sh}"
EXEC="${1:-hermes}"; RUNS="${2:-3}"
[ "$RUNS" -ge 1 ] 2>/dev/null || { echo "usage: boot-time.sh <executor> <runs>" >&2; exit 64; }
VM_XB="${VM_XB:-vm-xb}"
CATALOG_REPO_URL="${CATALOG_REPO_URL:-https://github.com/<organization>/agent-roster}"

echo "# B1 boot-time — executor=$EXEC runs=$RUNS — $(date -u)"
echo "| run | task_id | boot_s (spawn -> self-report.json) |"
echo "|---|---|---|"

times=()
boot_ms_arr=()
for i in $(seq 1 "$RUNS"); do
  id="b1-$(date +%H%M%S)-$i"
  start="$(date +%s)"
  bash "$RUN_TASK" "{\"task_id\":\"$id\",\"executor\":\"hermes\",\"agent_ref\":\"ephemeral-org/eph-backend-engineer\",\"agent_repo_url\":\"$CATALOG_REPO_URL\",\"repo_folder\":\"ephemeral-org/eph-backend-engineer\",\"repo_target\":\"repos/ephemeral-poc-demo\",\"behavioral_spec\":\"boot-time probe (M3 B1)\",\"requester\":\"m3-b1\",\"ttl_s\":300}" &
  runpid=$!
  # poll for /work/self-report.json on the spawned container (poc.task label)
  boot=""
  for _ in $(seq 1 400); do
    # FIX (B1): the executor's boot is FAST (~2-3s: clone -> self-report -> exit)
    # — a 2s poll cadence races the whole container lifetime and misses it.
    sleep 0.3
    if ssh "$VM_XB" "grep -aq boot_ms /var/poc/tasks/'$id'.log 2>/dev/null"; then
      # metric = spawn -> self-report ready: take the in-container measurement
      boot_ms=""
      boot_ms=$(ssh "$VM_XB" "grep -o '\"boot_ms\": \"[0-9]*\"' /var/poc/tasks/'$id'.log 2>/dev/null | head -1 | tr -dc 0-9")
      dispatch_s=$(( $(date +%s) - start ))
      if [ -n "$boot_ms" ]; then boot="$boot_ms ms (dispatch ${dispatch_s}s)"; else boot="${dispatch_s}s (dispatch-latency only)"; fi
      break
    fi
  done
  [ -n "$boot" ] || boot="n/a"
  [ -n "$boot" ] && times+=("$boot")
  boot_ms_arr+=("$boot_ms")
  wait "$runpid" || true
  echo "| $i | $id | $boot |"
done

# p50 of the measured boot times
p50="$(printf '%s\n' "${times[@]:-0}" | sort -n | awk '{a[NR]=$1} END{print (NR%2)?a[(NR+1)/2]:int((a[NR/2]+a[NR/2+1])/2)}')"
echo ""
echo "p50 boot time: ${p50} ms (in-container spawn -> self-report ready) — record the acceptance bound, re-run to pass (M3 §4 B1)"
echo "This p50 feeds M5's ttl_s sizing: ttl_s = max(3 x B1 p50, brief estimate) (M5 §1.4)."
