# M3 Installation & Deployment — Agent Executor Proofs (Hermes Ephemeral + Claude Code)

**Companion:** `../delivery plans/M3-Executor-Spikes-Delivery-Plan.md` · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | vm-xb (two executor images + token issuance on the lifecycle host); the Available Agent List + Project Repos on GitHub |
| **Prereqs** | M2 gate passed (lifecycle + worker + reaper healthy); GitHub access (owner's account) for repos + a GitHub App. **Sizing:** resize `vm-xb` to the recommended profile (12–16 GB / 6–8 cores / 60–80 GB) before this milestone if it isn't already — real executor containers are the main RAM/disk consumers (~2 GB per concurrent executor, cap 5, plus pinned images on disk). See the sizing table in M0 section 1. |
| **What gets deployed** | `poc/hermes-ephemeral:1.0.0`, `poc/claude-coder:1.0.0` images; token-issuance script; the agent-catalog repo; per-task branch strategy |
| **Language (mandatory)** | All Temporal code in this POC is Go (M1/M2 — untouched here; owner instruction: speed + size). The executor images run their own runtimes (Hermes runtime; node for the Claude Code CLI) — tooling, not Temporal SDK code, so outside the Go mandate. |

## 1. GitHub-side setup (do once, on the owner's workstation)

*Why:* the GitHub App is the R4 credential mechanism — a **scoped token issued at spawn and revoked at reap** means no long-lived owner secret ever enters a container. The permission split (read on the catalog, read+write only on the project repo) is least-privilege by design, and the App key living at `/opt/poc/secrets/app-key.pem` (chmod 600, never in git or an image) is the control the M5 hardening scan verifies.

**1a. Agent catalog repo — "Available Agent List"** (this IS the repo layout from M3 WP-3a.1):

```
available-agent-list/            # repo, read-only to agents
  index.json                     # agent index: [{ref, name, role, folder, skills[], spec_ref}]
  agents/
    dev-agent/
      persona.md                 # full agent persona
      skill-and-tools.md         # skill and tools the agent can pull the markdown files for
      behavioral-spec.md         # the Behavioral Specification (Note 1)
```

Create the repo + one worked example agent; commit.

**1b. Project Repo** — the repo the coder writes to; one POC project (Go or the POC's chosen language), CI off for the POC.

**1c. GitHub App (the token mechanism, R4):**
- Owner creates a GitHub App (repo: `ephemeral-agents-poc`), grants: `contents: read+write` on **Project Repo**, `contents: read` on **Available Agent List** only.
- Generate the App **private key** (PEM) → store on vm-xb at `/opt/poc/secrets/app-key.pem` (`chmod 600`, **never in git or any image**).
- Install the App to the owner's account → record `installation_id`.
- Issue at spawn time (M2's `spawn` calls this; the token lives only in the container's env):

```bash
# deploy/m3/issue-token.sh  (runs on vm-xb; JWT-signed, curl + openssl, ~20 lines)
# 1) JWT: RS256, aud=https://api.github.com/app, iat/exp (15 min)
# 2) POST /app/installations/{installation_id}/access_tokens   → installation token (TTL 1 h max)
# 3) echo the token → injected as GITHUB_TOKEN env by the lifecycle spawn
```

**Revocation on reap:** the lifecycle `reap` step calls `DELETE` on the token (GitHub App tokens can be force-expired) — the "unusable after reap" check in section 4 relies on this.

## 2. Build the two executor images

*Why:* the two entrypoint contracts are what everything after M3 asserts against: Hermes's **boot-time hydration + self-report** is Note 2 made concrete (M4's hydrate feeds it; M5's E2E checks it), and the coder's **per-task branch + commit + report** is the artifact chain the engineer's report points at. The executors run their own runtimes — tooling, not Temporal SDK code — so they sit *outside* the Go mandate, which is recorded here so nobody "fixes" them into it.

**2a. Hermes ephemeral** — `deploy/m3/hermes-ephemeral/`:

```dockerfile
FROM debian:trixie-slim               # or the estate's pinned base; record in image-tags.env
RUN apt-get update && apt-get install -y --no-install-recommends \
      ca-certificates git curl gnupg procps && rm -rf /var/lib/apt/lists/*
# Hermes runtime install (per the owner's Hermes packaging):
# HERMES_INSTALLER_URL is a mandatory build-arg: the estate's pinned Hermes
# installer URL+revision. The build FAILS if unset (no silent defaults).
RUN test -n "$HERMES_INSTALLER_URL" && curl -fsSL "$HERMES_INSTALLER_URL" | bash
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
```

`entrypoint.sh` (the boot-time hydration contract from Note 2 — "pull the full agent profile markdowns … load those profiles … then run the required tasks"):

1. `git clone --depth 1` the Available Agent List (auth: the read token from section 1c) into `/work/agents`; remove the token from env when done.
2. Resolve `AGENT_REF` via `index.json` → load `persona.md` + `skill-and-tools.md` + `behavioral-spec.md` for that agent's folder.
3. **Self-report** (gate before task work): write `/work/self-report.json` `{task_id, role, skills[], spec_received, sources{agent_repo, llm-wiki, hindsight}}` — the harness asserts on this file (key name `llm-wiki` per `deploy/m3/hermes-ephemeral/entrypoint.sh`; the entrypoint shallow-clones the llm-wiki git repo at boot).
4. Missing repo / malformed persona / index miss → write `self-report.json` with `error` and exit 2 (M2 surfaces + reaps).
5. Launch the Hermes agent runtime with persona + spec; on completion write a task report, exit 0.

Env (injected per-run by M2 spawn): `TASK_ID`, `AGENT_REPO_URL`, `AGENT_REF`, `REPO_FOLDER`, `BEHAVIORAL_SPEC`, `GITHUB_READ_TOKEN`, `HINDSIGHT_URL`, `LLM_WIKI_REPO_URL`, `LLM_WIKI_READ_TOKEN`, `TTL_S`. (`GITHUB_READ_TOKEN` is a **read-only** credential for catalog/repo clones; the coder's separate scoped write token is issued as `GITHUB_TOKEN` per section 1c — two different tokens, two different scopes. The llm-wiki is **not an HTTP service** (D-wiki-git): the Hermes entrypoint shallow-clones the git repo at boot via `LLM_WIKI_REPO_URL` (default `the owner-curated `llm-wiki` git repository (private; access requires authorization)`) using the static `LLM_WIKI_READ_TOKEN` (fine-grained PAT, `contents:read` on arvisdev/llm-wiki only); absent token → clone skipped, hydration degrades gracefully.)

**2b. Claude Code coder** — `deploy/m3/claude-coder/`:

```dockerfile
FROM node:22-slim                    # pin minor
RUN apt-get update && apt-get install -y --no-install-recommends git ca-certificates procps && rm -rf /var/lib/apt/lists/*
RUN npm install -g @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION}   # mandatory build-arg: exact CLI version (build fails if unset)
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
```

`entrypoint.sh`:
1. `git clone` Project Repo to `/work/project` (auth: `GITHUB_TOKEN`, the scoped App installation token from section 1c).
2. `git checkout -b task/<TASK_ID>` (per-task branch — the default concurrency model, section 3).
3. Run the headless agent: `claude -p "<task prompt + behavioral spec + design standards retrieved at hydration>" --max-turns N` with the API key from `ANTHROPIC_API_KEY` env (injected per-run, scoped by the owner's billing/limits; **never baked into the image**). Hermes's own model-provider key(s) ride the same way via the M2 spawn env (`MODEL_API_KEY`) — required at spawn, never in the image.
4. `git add -A && git commit -m "task/<TASK_ID>: <summary>" && git push origin task/<TASK_ID>`.
5. Write `/work/report.json` `{task_id, branch, commit_sha, files_changed}`; exit 0 (non-zero on push failure).

**Build + load onto vm-xb** (same transfer pattern as M1/M2):

```bash
docker build -f deploy/m3/hermes-ephemeral/Dockerfile \
  --build-arg HERMES_INSTALLER_URL="$HERMES_INSTALLER_URL" \
  deploy/m3/hermes-ephemeral -t poc/hermes-ephemeral:1.0.0
docker build -f deploy/m3/claude-coder/Dockerfile \
  --build-arg CLAUDE_CODE_VERSION="$CLAUDE_CODE_VERSION" \
  deploy/m3/claude-coder -t poc/claude-coder:1.0.0
docker save poc/hermes-ephemeral:1.0.0 poc/claude-coder:1.0.0 | ssh vm-xb docker load
# update M2: POC_IMAGE per task type in deploy/m2/compose.yaml + lifecycle config
```

Both build-args are mandatory (the Dockerfiles fail the build if unset — see section 2's
`RUN test -n "$HERMES_INSTALLER_URL"` guard and the `CLAUDE_CODE_VERSION` note);
export them in the build shell before running the block.

## 3. Concurrency model — install decision (R3)

*Why:* the concurrency decision is made **here, not at M5**, because M3 is its last cheap point: the N-parallel option is an M2 *config* change (worktree fan-out), not new infrastructure. Choosing it later, after hardening, means re-hardening the whole flow. Either way the choice is recorded in the decision log — it's an R3 closure, not an assumption.

- **Owner input (2026-10-25) — the semantics:** container count is **needs-based, not fixed** — a Hermes agent when agent work is needed, a Claude Code container when coding occurs, and **multiple containers in parallel when requests are concurrent**. The team sets a **hard limit in configuration**; the POC rule of thumb is **max 5**, tuned to the VM's configuration (a guideline, not an absolute). What remains to be chosen below is the *execution* model (branch vs. worktree) and the concrete cap value, under that limit.

- **Default (recorded decision):** one coder per task → one container, one `task/<id>` branch, no shared checkout. M2 spawns a single container.
- **If N-parallel is chosen later:** same image, per-coder **git worktrees** (each container gets `/work/project-<i>` via `git worktree add` on a shared base clone) — add an `N` field to the task config and fan out N spawns from "Executes Activity". This is a **change to M2 config, not a new image**.

## 4. Verification (the M3 gate tests)

*Why:* M3 is the **point of last cheap exit** — if the two To-verify components fail here, the cost is a two-week spike rewrite, not a re-architecture of M4/M5. B1's boot-time p50 matters beyond the gate: it's the measured number M5's TTL sizing uses. The evidence table + self-reports are what `TP-M3` signs.

```bash
# B1 boot-time (3 runs): spawn via M2 with a stub task; time: container start → self-report.json ready
./tests/boot-time.sh hermes 3            # record p50; decide the acceptance bound, then re-run to pass
# B2 persona correctness: self-report.json shows role/skills/spec matching agents/dev-agent/
# B3 failure paths: AGENT_REPO_URL unreachable → exit 2 + reap; malformed persona → exit 2 + reap
# B4 coder real work: M3b task "add a /healthz handler" → commit on task/<id>, pushed
git log --oneline origin/task/<id>        # owner's workstation: the commit exists with files_changed
# B5 token lifecycle: after reap, the issued token is revoked/expired (force-check with curl → 401)
# B6 (only if N-parallel chosen) 2 concurrent coders → 2 branches, both push, no lost work
```

Evidence → `deploy/m3/evidence/` (boot-time table, self-reports, commit SHAs, token check output).

## 5. Secret hygiene checks (R4, run before claiming done)

*Why:* "no secrets baked in" is the R4 property M5's hardening re-verifies fleet-wide — the three checks here (env dump, image history, trivy secrets class) are exactly the evidence that proves injection-at-spawn really happens. An image that fails this never ships; the fix is rebuilding the layer that wrote the secret, not deleting the string.

```bash
docker run --rm --entrypoint env poc/hermes-ephemeral:1.0.0 > /tmp/env1; docker run --rm --entrypoint env poc/claude-coder:1.0.0 > /tmp/env2   # --entrypoint: run env directly, don't trigger the entrypoint's exit-2
grep -Ei 'token|key|secret' /tmp/env1 /tmp/env2          # expect: NOTHING (secrets are injected at spawn, not baked in)
docker history --no-trunc <images> | grep -Ei 'secret|token'   # expect: no RUN layer wrote secrets
trivy image --exit-code 1 poc/hermes-ephemeral:1.0.0 poc/claude-coder:1.0.0   # secrets vuln class = 0
```

## 6. Rollback / re-do

*Why:* `POC_IMAGE` is a **config switch, not a code change** — M2 keeps its stub pin until the new executor tags pass, so a bad M3 image can never take down the lifecycle. GitHub-side rollback (uninstall App, delete tokens) is independent of the repos, which stay untouched.

```bash
# images: rebuild + re-load; M2 keeps the M2 stub image pinned until the new tags pass, so a bad M3
# image never takes down the lifecycle (POC_IMAGE is a config switch, not a code change)
# GitHub side: uninstall the App + delete tokens; catalog/project repos are untouched
```

## 7. Troubleshooting

*Why:* each row is a pre-diagnosed failure mode with a one-line cause — the classic ones being token scope (section 1c), `index.json` ref mismatch, and TTL < task runtime ("measure, don't guess" is the M3 discipline: the TTL comes from B1's measured p50, not from a guess).

| Symptom | Cause / fix |
|---|---|
| Hermes boot hangs at "loading persona" | repo clone failing (token wrong repo scope — section 1c) or index.json ref mismatch (`AGENT_REF` vs `index.json`) |
| `claude` CLI exits before doing work | `ANTHROPIC_API_KEY` missing/expired in the run env; CLI version pin too old for the flag set — pin + record |
| Push rejected on task branch | App install missing `contents:write` on the Project Repo; or branch protection on the POC repo (off in section 1b) |
| Reaper doesn't get the coder container | TTL < task runtime: the coder's `TTL_S` must exceed `max-turns` time budget — measure, don't guess |
| Token not revocable | GitHub App installation tokens expire at 1 h; for earlier revocation use App token DELETE endpoint (section 1c step 3) |
