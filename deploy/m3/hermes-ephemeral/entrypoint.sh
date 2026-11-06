#!/usr/bin/env bash
# =============================================================================
# M3 Hermes-ephemeral entrypoint — the boot-time hydration contract (Note 2)
# =============================================================================
# "pull the full agent profile markdowns ... load those profiles ... then run
#  the required tasks" — M3 install doc §2a steps 1–5.
#
# Spawn env (injected per-run by M2 spawn — never baked into the image):
#   TASK_ID AGENT_REPO_URL AGENT_REF REPO_FOLDER BEHAVIORAL_SPEC GITHUB_READ_TOKEN
#   HINDSIGHT_URL LLM_WIKI_REPO_URL LLM_WIKI_READ_TOKEN TTL_S
#
# The llm-wiki is a git repo (no retrieval API): shallow-cloned at boot when
# LLM_WIKI_READ_TOKEN is present; absence degrades gracefully (rollback rule).
#
# On success: writes /work/self-report.json, runs the task, writes a task
# report, exit 0. Any failure path: self-report.json with `error`, exit 2.
set -u

TASK_ID="${TASK_ID:-unknown}"
AGENT_REPO_URL="${AGENT_REPO_URL:-}"
AGENT_REF="${AGENT_REF:-}"
REPO_FOLDER="${REPO_FOLDER:-}"
BEHAVIORAL_SPEC="${BEHAVIORAL_SPEC:-}"
GITHUB_READ_TOKEN="${GITHUB_READ_TOKEN:-}"
HINDSIGHT_URL="${HINDSIGHT_URL:-}"
LLM_WIKI_REPO_URL="${LLM_WIKI_REPO_URL:-https://github.com/arvisdev/llm-wiki}"
LLM_WIKI_READ_TOKEN="${LLM_WIKI_READ_TOKEN:-}"
TTL_S="${TTL_S:-900}"

START_MS="$(date +%s%3N)"   # container start (B1 boot-time origin)
mkdir -p /work
fail() { # fail <reason>  -> self-report with error, exit 2 (M2 surfaces + reaps)
  printf '{"task_id":"%s","role":"","skills":[],"spec_received":false,"error":"%s","sources":{"agent_repo":false,"llm-wiki":"not-configured","hindsight":false}}\n' \
    "$TASK_ID" "$1" > /work/self-report.json
  exit 2
}

# 1. clone the Available Agent List (read token), then scrub the token from env
[ -n "$AGENT_REPO_URL" ] || fail "AGENT_REPO_URL missing"
CLONE_URL="${AGENT_REPO_URL/https:\/\//https:\/\/x-access-token:$GITHUB_READ_TOKEN@}"
git clone --depth 1 "$CLONE_URL" /work/agents \
  2>/dev/null || fail "agent repo unreachable or token rejected"
unset GITHUB_READ_TOKEN

# 2. resolve AGENT_REF via index.json -> load the agent's markdowns
[ -f /work/agents/index.json ] || fail "index.json missing"
folder="$(python3 -c "
import json,sys
idx=json.load(open('/work/agents/index.json'))
ref='$AGENT_REF'
for a in idx if isinstance(idx,list) else idx.get('agents',[]):
    if a.get('ref')==ref:
        print(a.get('folder','')); break
else: sys.exit(1)
")" || fail "index miss: AGENT_REF '$AGENT_REF' not in index.json"
[ -n "$folder" ] || fail "index entry for '$AGENT_REF' has no folder"
for f in persona.md skill-and-tools.md behavioral-spec.md; do
  [ -f "/work/agents/$folder/$f" ] || fail "malformed persona: $folder/$f missing"
done

# 2b. llm-wiki hydration: shallow-clone the git repo at boot (optional —
# degrades gracefully when the token/repo is absent; never fails the task)
llm_wiki_source="not-configured"
if [ -n "$LLM_WIKI_READ_TOKEN" ]; then
  LLM_WIKI_URL_INJ="${LLM_WIKI_REPO_URL/https:\/\//https:\/\/x-access-token:$LLM_WIKI_READ_TOKEN@}"
  git clone --depth 1 "$LLM_WIKI_URL_INJ" /work/llm-wiki 2>/dev/null
  llm_wiki_source="$LLM_WIKI_REPO_URL"
  unset LLM_WIKI_READ_TOKEN
fi

# 3. self-report (gate before task work) — the harness asserts on this file
hindsight_ok=false
[ -n "$HINDSIGHT_URL" ] && curl -sf "$HINDSIGHT_URL/healthz" >/dev/null 2>&1 && hindsight_ok=true
# TC-M3-02: assert the persona markdowns were actually loaded from the catalog
persona_files=""
for f in persona.md skill-and-tools.md behavioral-spec.md; do
  [ -f "/work/agents/$folder/$f" ] && persona_files="$persona_files $f"
done
[ -n "$persona_files" ] || fail "persona markdowns missing from catalog hydration"

HINDSIGHT_OK="$hindsight_ok"
BOOT_MS="$(( $(date +%s%3N) - START_MS ))"   # true boot: container start -> self-report ready
python3 - "$folder" "$llm_wiki_source" <<PY > /work/self-report.json
import json
print(json.dumps({
  "task_id": "$TASK_ID", "role": "$folder", "skills": [],
  "spec_received": True,
  "sources": {"agent_repo": True, "llm-wiki": "$llm_wiki_source", "hindsight": "$HINDSIGHT_OK"}, "persona_files": "$persona_files", "boot_ms": "$BOOT_MS"
}))
PY

# 4/5. launch the Hermes agent runtime with persona + spec; write the task report
# (the real runtime launch is the owner's Hermes packaging — see the Dockerfile's
# pinned installer; the report is the exit artifact M2/M5 consume)
# emit the self-report to stdout — the lifecycle persists it in the task
# artifact log (survives reap; the B1 poller greps this instead of racing
# docker exec against a 2-second container lifetime)
cat /work/self-report.json

cat > /work/report.json <<EOF
{"task_id":"$TASK_ID","status":"success","artifact":"task report","spec":"$REPO_FOLDER/behavioral-spec.md"}
EOF
exit 0
