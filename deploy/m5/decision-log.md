# M5 decision log — open items locked (2026-11-07)

- [x] **VM numbering (carried from M0)** — final scheme recorded: 1000 vm-ephemeral-agents, 1001 vm-docker-registry, 1500 vm-keycloack, 2000 vm-hermes-agent, 2001 vm-hindsight, 3002 vm-n8n (owner-managed, stopped). Estate-wide ens18 IP reassignment applied 2026-11-07 (.223/.198/.224/.233); all repo refs, compose files, daemon.json and hosts.env updated (commit 1b72825).
- [x] **Topic/queue message shapes** — `schemas/message-v1.json` is the contract; versioned by filename. The manager composes the work package into `behavioral_spec` (spec content + task brief, Note-1 job); the lifecycle splits `agent_ref` (URL#folder) into AGENT_REF + AGENT_REPO_URL at the M2/M3 boundary. No correlation_id; task_id is the sole key.
- [x] **Concurrency model** — **one-coder-per-task** (M3 §3 default): each task gets a dedicated `task/<task_id>` branch; sequential tasks proven by TC-M3-07 (tc07-seq-1/2/3b markers landed in order). N-parallel + worktrees is a recorded **V2 open item**.
- [x] **Ephemeral isolation level** — co-located containers on vm-xb for the POC (network `poc-net`, label-driven reaper, zero-survivors gate). Per-task VMs / stronger isolation explicitly **deferred to V2** (isolation level was an open M2/M4 question; decided: defer).
- [x] **llm-wiki repo curation owner + cadence (from M4)** — D-curation (2026-11-04): owner = the Human-In-The-Loop (HITL); append-only, need-driven; recorded in the llm-wiki repo's CHANGELOG.md.

## POC-closure decisions recorded during M5 (2026-11-07)

- **Executor default**: `POC_EXECUTOR_DEFAULT=hermes` (stub remains the rollback pin). The M5 §4 acceptance ran on the hermes executor (hydration + report chain); TC-M5-02(c)'s commit evidence was produced by a coder-routed acceptance request through the identical gateway→manager→queue path (commit 962c3ce on `task/364022fd-4ed1-46b7-9c02-015a3ee9a9ca`, /healthz handler verified on a fresh clone).
- **Entrypoint TASK_PROMPT guard** (root cause, run 16→17): the hermes entrypoint referenced `$TASK_PROMPT` under `set -u`; spec-composed work packages leave task_prompt empty, so the lifecycle now coalesces empty TaskPrompt→BehavioralSpec and the entrypoint guards the var (commit 21cac07).
- **llm-proxy canonicalized** (root cause, run 17 coder retry): the sanitize fix lived only in `/tmp/proxy.js` on vm-xb and was lost on redeploy → OpenRouter 422 `is_error` returned. Proxy v3 (sanitize + provider pin + /hello) is now in-repo at `deploy/m3/llm-proxy/proxy.js` (commit 607b5f6); never configure it from /tmp again.
- **M0 gate-2 idempotent re-run race**: the cross-VM publish test counts queue depth 2 s after publish; with live consumers the durable queue drains instantly. Proven live during M5 (publish `routed:true`, delivered counter 60→61) — test-design race, not a defect; M0 stays closed.
- **§4 gate hardened**: the acceptance check now dies unless the report `status` is `success` (run 16's "artifact chain verified" only checked reaped cleanliness — closed; commit 21cac07).
- **§6 accepted residuals (per the M3 TC-M3-06 precedent)**: secrets=0 and fixable HIGH/CRITICAL=0 are hard gates; unfixed base-image OS CVEs and the pinned third-party CLIs' bundled dependency trees (Claude Code CLI 2.1.318, Hermes Agent v0.23.0 lang-pkgs) are recorded residuals — fixes live upstream, not in these images.
- **Embeddings interim** (from M4): `EMBEDDINGS=none` (no embeddings provider available in the estate); NaN-safe zero-vector fallback. **V2 open item**.

## Version 2 open items list

1. Multi-flow concurrency: N-parallel coders + git worktrees (M3 §3 alternative).
2. Ephemeral isolation level: per-task VMs or stronger sandboxing vs. co-located containers.
3. Embeddings provider for Hindsight semantic retrieval (none available in the estate today).
4. Long-term queue shape and topic/queue schema evolution beyond message-v1.
5. Key vault / secrets management (fine-grained PATs, per-VM vault) — interim gitignored m3.env is POC-only.
6. llm-wiki curation automation beyond the HITL append-only cadence.
7. Upstream dependency refreshes: Claude Code CLI >2.1.318 and Hermes Agent >v0.23.0 to clear the bundled-dep residuals.
8. ARCHITECTURE.md / Initial-Foundation-Design-Document.md topology diagrams to be re-drawn by the owner after design validation (intentionally untouched by automation).

## Post-close decision (2026-11-08): catalog switched to agent-roster

- **D-catalog-realization (2026-11-08):** the machine-readable catalog the
  manager matches against and the executors hydrate from is now
  **`agent-roster`** (AGENT ROSTER (ephemeral-org)) — replacing the planned
  Available Agent List. The roster repository carries: a root `index.json`
  (22 eph-* entries in the wrapped `{"agents":[…]}` shape, fields
  ref/name/role/folder/skills/spec_ref) and, per eph-* folder, the three
  hydration files the executor contract requires (`persona.md`,
  `skill-and-tools.md`, `behavioral-spec.md`). Rationale: one source of
  truth serves both the human staff directory (the roster documents) and
  the machine catalog — no "documentation copy" that can drift.
- The version-1 contract doc (`version 1/Available-Agent-List-Repository.md`)
  remains the baselined design record; this entry is the realizing decision
  per the version-closure rule (nothing is edited inside a baselined doc set).
- **Estate applied:** m5-manager `AGENT_LIST_REPO` switched to
  `agent-roster.git` and the container recreated; live smoke verified —
  capability "risk assessment" routed to
  `ephemeral-org/eph-risk-assessment-analyst`, the executor hydrated all
  three persona files (boot 1457 ms), report `status: success`.
- Repo commits: agent-roster `f9ec562` (catalog files), ephemeral-agents
  `a1e7dea` (references), README/comment sync this pass.
