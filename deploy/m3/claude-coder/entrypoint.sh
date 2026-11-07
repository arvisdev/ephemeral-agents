#!/usr/bin/env bash
# =============================================================================
# M3 Claude Code coder entrypoint — the artifact chain contract (M3 §2b 1–5)
# =============================================================================
# Spawn env (injected per-run by M2 spawn — never baked into the image):
#   TASK_ID REPO_URL GITHUB_TOKEN (scoped App installation token, §1c)
#   TASK_PROMPT BEHAVIORAL_SPEC ANTHROPIC_API_KEY
set -u

TASK_ID="${TASK_ID:-unknown}"
REPO_URL="${REPO_URL:-}"
GITHUB_TOKEN="${GITHUB_TOKEN:-}"
TASK_PROMPT="${TASK_PROMPT:-}"
BEHAVIORAL_SPEC="${BEHAVIORAL_SPEC:-}"

mkdir -p /work

# 1. clone the Project Repo with the scoped write token
[ -n "$REPO_URL" ] && [ -n "$GITHUB_TOKEN" ] || { echo "REPO_URL/GITHUB_TOKEN missing" >&2; exit 2; }
CLONE_URL="${REPO_URL/https:\/\//https:\/\/x-access-token:$GITHUB_TOKEN@}"
git clone "$CLONE_URL" /work/project \
  2>/dev/null || { echo "clone failed (token scope? §1c)" >&2; exit 2; }

# 2. per-task branch — the default concurrency model (§3: one coder per task)
git -C /work/project checkout -b "task/${TASK_ID}" || exit 2

# 3. run the headless agent (API key injected per-run, never in the image)
[ -n "$ANTHROPIC_API_KEY" ] || { echo "ANTHROPIC_API_KEY missing at spawn" >&2; exit 2; }
MODEL="${MODEL:-${MODEL_DEFAULT:-}}"   # owner directive: default z-ai/glm-5.3-flash via openrouter, never anthropic
# gateway/custom model ids must be exposed through a known alias (claude-code docs:
# model-config#correct-the-window-for-a-gateway-or-custom-model-id); a raw id as --model
# triggers [claude-code:unrecognized_model] and the SDK exits nonzero.
export ANTHROPIC_DEFAULT_SONNET_MODEL="$MODEL"
MODEL_ALIAS="sonnet"
cmd=(claude -p "${TASK_PROMPT}

Behavioral spec:
${BEHAVIORAL_SPEC}" --allowedTools "Edit,Write,MultiEdit,Read,Glob,Grep,Bash" --max-turns "${MAX_TURNS:-20}")
if [ -n "$MODEL_ALIAS" ]; then cmd+=(--model "$MODEL_ALIAS"); fi
summary="$("${cmd[@]}")" || { echo "claude CLI failed" >&2; exit 2; }

# 4. commit + push the per-task branch
export GIT_AUTHOR_NAME="ephemeral-coder" GIT_AUTHOR_EMAIL="coder@poc.local" \
       GIT_COMMITTER_NAME="ephemeral-coder" GIT_COMMITTER_EMAIL="coder@poc.local"
git -C /work/project add -A
git -C /work/project commit -m "task/${TASK_ID}: ${summary}" || true
git -C /work/project push origin "task/${TASK_ID}" || { echo "push failed (contents:write? branch protection?)" >&2; exit 2; }

# 5. report (artifact chain: branch + commit + files changed)
commit_sha="$(git -C /work/project rev-parse HEAD)"
files_changed="$(git -C /work/project diff --name-only "origin/main...HEAD" 2>/dev/null | tr '\n' ',' | sed 's/,$//')"
printf '{"task_id":"%s","branch":"task/%s","commit_sha":"%s","files_changed":"%s"}\n' \
  "$TASK_ID" "$TASK_ID" "$commit_sha" "$files_changed" > /work/report.json
exit 0
