#!/usr/bin/env bash
# M2 stub-executor entrypoint — M2 install doc §1a:
# "echo back the config env (TASK_ID, AGENT_REF, BEHAVIORAL_SPEC) to stdout,
#  create a marker file, then sleep $TTL (responds to SIGTERM)."
# Inject config (not secrets) via M2 spawn env; V2 injects failure, V3 hangs.
set -u

TASK_ID="${TASK_ID:-unknown}"
AGENT_REF="${AGENT_REF:-}"
BEHAVIORAL_SPEC="${BEHAVIORAL_SPEC:-}"
TTL="${TTL_S:-${TTL:-${POC_TTL_DEFAULT:-900}}}"   # lifecycle passes TTL=<seconds> (doc §1a); TTL_S only as override
TTL="${TTL%s}"   # accept "900s"-style duration strings

echo "{\"event\":\"config\",\"task_id\":\"$TASK_ID\",\"agent_ref\":\"$AGENT_REF\",\"behavioral_spec\":\"$BEHAVIORAL_SPEC\",\"ttl\":\"${TTL}s\"}"

mkdir -p /work
echo "started $(date -u +%FT%TZ)" > /work/marker

# graceful SIGTERM handling (docker stop / reap must not leak)
term() { echo "{\"event\":\"sigterm\",\"task_id\":\"$TASK_ID\"}"; exit 0; }
trap term TERM INT

case "${INJECT:-}" in
  exit-1) echo "{\"event\":\"fail\",\"task_id\":\"$TASK_ID\",\"reason\":\"injected exit-1\"}"; exit 1 ;;
  # FIX (run8): hang must survive PAST poc.expires or the TTL backstop never
  # fires — sleep 10x the TTL (the reaper rm -f / activity reap ends it).
  hang)   echo "{\"event\":\"hang\",\"task_id\":\"$TASK_ID\",\"until\":\"$((TTL * 10))s\"}" ; TTL=$((TTL * 10)) ;;
  *)      echo "{\"event\":\"run\",\"task_id\":\"$TASK_ID\"}" ;;
esac

sleep "${TTL:-900}" &
wait $!
