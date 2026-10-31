#!/usr/bin/env bash
# =============================================================================
# M1 test harness — assertions for the T1–T5 blocks (M1 install doc §4)
# =============================================================================
# Usage:
#   assert.sh eq   <description> <actual>   <expected>   # string/number equality
#   assert.sh ge   <description> <actual>   <expected>   # numeric >=
#   assert.sh queue <queue> <want>          # mgmt-API queue message count == want
#   assert.sh dlq   <want>                  # agent.tasks.dlq count == want
#
# Exit 0 = PASS, 1 = FAIL; prints "<description> : PASS/FAIL" — the T-blocks
# and TP-M1 evidence capture these lines.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
RMQ_API_BASE="${RMQ_API_BASE:-http://127.0.0.1:15672}"
RMQ_SSH_HOST="${RMQ_SSH_HOST:-}"
RMQ_ENV_FILE="${RMQ_ENV_FILE:-$REPO_ROOT/deploy/m0/rabbitmq/.env}"

pass() { echo "$1 : PASS"; }
fail() { echo "$1 : FAIL"; return 1; }

queue_count() { # queue_count <queue-name>
  local passw
  passw="$(grep '^RABBITMQ_PASS=' "$RMQ_ENV_FILE" | cut -d= -f2)"
  if [ -n "$RMQ_SSH_HOST" ]; then
    printf 'user = "poc:%s"\n' "$passw" | ssh "$RMQ_SSH_HOST" \
      "curl -s -K - '${RMQ_API_BASE}/api/queues/%2F/$1'"
  else
    curl -s -u "poc:$passw" "${RMQ_API_BASE}/api/queues/%2F/$1"
  fi | grep -o '"messages":[0-9]*' | cut -d: -f2 || echo 0
}

mode="${1:-}"; shift || true
case "$mode" in
  eq) d="$1" a="$2" e="$3"; [ "$a" = "$e" ] && pass "$d ($a)" || fail "$d (got $a, want $e)" ;;
  ge) d="$1" a="$2" e="$3"; [ "$a" -ge "$e" ] 2>/dev/null && pass "$d ($a >= $e)" || fail "$d (got $a, want >= $e)" ;;
  queue) q="$1" w="$2"; n="$(queue_count "$q")"; [ "${n:-0}" = "$w" ] && pass "queue $q = $w" || fail "queue $q = ${n:-0}, want $w" ;;
  dlq) w="$1"; n="$(queue_count agent.tasks.dlq)"; [ "${n:-0}" = "$w" ] && pass "dlq = $w" || fail "dlq = ${n:-0}, want $w" ;;
  *) sed 's/^# \{0,1\}//' "$0"; exit 64 ;;
esac
