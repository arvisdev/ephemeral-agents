#!/usr/bin/env bash
# =============================================================================
# M1 test harness — publish task messages to the agent.tasks topic
# =============================================================================
# Implements M1 install doc §4 (T1–T5) and the M1 delivery plan WP-1.4:
#   publish.sh [N]        publish N distinct valid message-v1 task messages
#   publish.sh N --dup    republish the SAME task_ids as the last N-run (T2)
#   publish.sh --invalid K  publish K schema-invalid poison messages (T4)
#
# Publishes through the RabbitMQ management API (loopback on vm-xb), which is
# what the M0 cross-VM publish test uses. Runs on vm-xb, or from anywhere with
# RMQ_SSH_HOST set (relayed via ssh) / RMQ_API_BASE pointed at the mgmt API.
#
# Env:
#   RMQ_API_BASE   default http://127.0.0.1:15672
#   RMQ_SSH_HOST   if set, all API calls relay through `ssh $RMQ_SSH_HOST`
#   RMQ_ENV_FILE   default <repo>/deploy/m0/rabbitmq/.env (RABBITMQ_PASS)
#   EXCHANGE       default agent.tasks   QUEUE default agent.tasks.durable
#   STATE_FILE     default <script dir>/.last-task-ids (enables --dup)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

EXCHANGE="${EXCHANGE:-agent.tasks}"
QUEUE="${QUEUE:-agent.tasks.durable}"
RMQ_API_BASE="${RMQ_API_BASE:-http://127.0.0.1:15672}"
RMQ_SSH_HOST="${RMQ_SSH_HOST:-}"
RMQ_ENV_FILE="${RMQ_ENV_FILE:-$REPO_ROOT/deploy/m0/rabbitmq/.env}"
STATE_FILE="${STATE_FILE:-$SCRIPT_DIR/.last-task-ids}"

api() { # api <method> <path> [json-body]
  local method="$1" path="$2" body="${3:-}"
  if [ -n "$RMQ_SSH_HOST" ]; then
    printf 'user = "poc:%s"\n' "$RABBITMQ_PASS" | ssh "$RMQ_SSH_HOST" \
      "curl -s -K - -X$method '${RMQ_API_BASE}${path}' ${body:+-H 'content-type: application/json' -d '$body'}"
  else
    curl -s -u "poc:$RABBITMQ_PASS" -X "$method" "${RMQ_API_BASE}${path}" \
      ${body:+-H 'content-type: application/json' -d "$body"}
  fi
}

publish() { # publish <payload-json> <routing-key>
  local payload="$1" rk="$2"
  api POST "/api/exchanges/%2F/$EXCHANGE/publish" \
    "{\"routing_key\":\"$rk\",\"payload\":$(printf '%s' "$payload" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))'),\"payload_encoding\":\"string\",\"properties\":{\"delivery_mode\":2}}" \
    | grep -q '"routed":true' || { echo "publish failed: $payload" >&2; return 1; }
}

N=0; DUP=0; INVALID=0
while [ $# -gt 0 ]; do
  case "$1" in
    --dup) DUP=1 ;;
    --invalid) INVALID="${2:?count}"; shift ;;
    -h|--help) sed 's/^# \{0,1\}//' "$0"; exit 0 ;;
    *) N="$1" ;;
  esac; shift
done

if [ ! -s "$RMQ_ENV_FILE" ]; then
  echo "missing $RMQ_ENV_FILE (run M0 install first: RABBITMQ_PASS)" >&2; exit 1
fi
# shellcheck disable=SC1090
RABBITMQ_PASS="$(grep '^RABBITMQ_PASS=' "$RMQ_ENV_FILE" | cut -d= -f2)"
[ -n "$RABBITMQ_PASS" ] || { echo "RABBITMQ_PASS empty in $RMQ_ENV_FILE" >&2; exit 1; }

# --invalid K (T4): poison messages — violate message-v1.json (missing required
# fields); the M1 adapter must route them to the DLQ with the reason as property.
if [ "$INVALID" -gt 0 ]; then
  for i in $(seq 1 "$INVALID"); do
    publish "{\"task_id\": \"not-valid-$i\"}" "$QUEUE"    # missing required fields
  done
  echo "published $INVALID invalid messages to $EXCHANGE (expect DLQ)"
  exit 0
fi

[ "$N" -gt 0 ] || { echo "usage: publish.sh [N] [--dup] [--invalid K]" >&2; exit 64; }

# task_ids: fresh UUIDs for a normal run; the SAME ids as the last run for --dup
if [ "$DUP" = "1" ]; then
  [ -s "$STATE_FILE" ] || { echo "--dup: no previous run in $STATE_FILE" >&2; exit 1; }
  mapfile -t IDS < "$STATE_FILE"
  [ "${#IDS[@]}" -eq "$N" ] || { echo "--dup: last run had ${#IDS[@]} ids, want $N" >&2; exit 1; }
  echo "republishing ${#IDS[@]} duplicate task_ids (expect: no new workflows)"
else
  IDS=()
  for i in $(seq 1 "$N"); do
    IDS+=("$(python3 -c 'import uuid;print(uuid.uuid4())')")
  done
  printf '%s\n' "${IDS[@]}" > "$STATE_FILE"
fi

for id in "${IDS[@]}"; do
  publish "{\"task_id\":\"$id\",\"agent_ref\":\"agents/dev-agent\",\"behavioral_spec\":\"m1 gate test task\",\"repo_target\":\"ephemeral-agents-poc\",\"requester\":\"m1-tests\",\"ttl_s\":300}" "$QUEUE"
done
echo "published $N messages to $EXCHANGE"
