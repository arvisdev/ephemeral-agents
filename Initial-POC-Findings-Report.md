# Initial POC Findings Report

**Project:** Ephemeral Agents — Enterprise Multi-Agent Orchestration POC
**Date:** 2026-11-07 · **Status:** ALL MILESTONES M0–M5 COMPLETE — POC BASELINE CLOSED
**Author:** Hermes Agent (operator), signed off by HITL (the project owner)
**Repository:** github.com/<organization>/ephemeral-agents (this report included; evidence dirs are gitignored by design)

---

## 1. Executive summary

The POC set out to prove — on real infrastructure, not mocks — that an enterprise
multi-agent organization can be stood up end to end: a chassis of VMs, a Temporal-
based durable task workflow, an agent catalog, ephemeral single-task agent
containers, cross-task memory, and a human front door. **All six milestones (M0–M5)
passed their test procedures and are signed.** The final gate (TP-M5) demonstrated a
real request from a human desktop becoming a durable workflow, an ephemeral coder
container performing a real GitHub commit (`962c3ce` on `task/364022fd…`, adding a
verified `/healthz` handler), a structured report delivered back to the human —
with zero surviving containers and no long-lived credentials anywhere.

**Verdict: technically feasible — now confirmed by evidence.**

## 2. Milestone results

| Milestone | Scope | TP result | Closed |
|---|---|---|---|
| M0 | Chassis: VMs, Docker, RabbitMQ topology, Temporal | TP-M0 PASS (3 gates) | 2026-11-05 |
| M1 | Topic bridge + adapter (RabbitMQ contract) | TP-M1 6/6 PASS | 2026-11-05 |
| M2 | Durable task lifecycle (Temporal TaskWorkflow, spawn/reap) | TP-M2 PASS (runs 1–10) | 2026-11-06 |
| M3 | Agent catalog + ephemeral executors (hermes + coder) | TP-M3 7/7 PASS | 2026-11-07 |
| M4 | Cross-task memory (Hindsight API + pgvector) | TP-M4 7/7 PASS | 2026-11-07 |
| M5 | Front-of-house + E2E + hardening + lockout | TP-M5 7/7 PASS | 2026-11-07 |

## 3. What was proven (evidence highlights)

- **E2E artifact chain (TC-M5-02):** desktop request → gateway → manager (capability
  resolution via `index.json`) → RabbitMQ → lifecycle → ephemeral executor → report
  JSON at `GET /gateway/reports/<task_id>`. Temporal trace verified; **zero
  survivors** after every run.
- **Real agent work:** the coder executor (Claude Code CLI 2.1.318 routed to
  `z-ai/glm-5.3-flash` via OpenRouter through the llm-proxy sidecar) cloned the
  project repo on a scoped, per-task GitHub App installation token, branched
  `task/<id>`, made a real edit, pushed commit `962c3ce`, and exited; the report
  carried the artifact pointer.
- **Credential hygiene (TC-M5-04):** no long-lived owner secret in any container env;
  per-task tokens are minted (JWT exp ≤ 540 s), scoped, and **dead within ~75 s of
  reaping** (HTTP 401 post-revoke, TC-M3-05/§6 re-checks).
- **Cross-task memory (TC-M4-02):** a later task's boot log contains prior tasks'
  learnings (`M4_MEMORY_HYDRATE`), retrieved from Hindsight in ~13 ms; the M5
  acceptance run hydrated the full memory/wiki/persona chain live.
- **Failure handling (TC-M5-03):** F1–F5 injections each ended in a clean reap with
  zero survivors and a human-visible error or a completed report; **F5 (manager
  restart mid-task) still wrote the report after restart**.
- **Registry-only operation:** every spawn sources images from the private registry
  (<registry-host>:5000) — the rmi-first proof (TC-M2-06 pattern) removed all local
  copies from vm-xb.

## 4. Performance figures

| Metric | Value |
|---|---|
| Boot time (persona hydration → self-report ready), B1 p50 | **785 ms** (785/802/751 ms, registry-backed) |
| Hindsight memory read latency | ~13 ms |
| Task repo clone (shallow) | 0.6–1.4 s |
| Full E2E acceptance (request → report, hermes executor) | ≈ 3 min (incl. ~1.7 s boot hydration) |
| Coder acceptance (spawn → pushed commit) | ≈ 3 min (single-turn edit via friendli-pinned route) |
| Token revocation lag (reap → 401) | ≤ 75 s |

## 5. Defect root-cause ledger (major items)

~35 defects were root-caused and fixed across the milestones; the most instructive:

1. **Env-load timing** — lifecycle env was read before the secrets mount settled →
   order the mount before env load.
2. **JWT exp 900 s → 540 s** — GitHub caps App JWTs at 600 s; minting silently failed.
3. **`REPO_URL` contract** — `https://github.com/<organization>/` + `TrimPrefix(RepoTarget,"repos/")`.
4. **Model aliasing** — `--model sonnet` + `ANTHROPIC_DEFAULT_SONNET_MODEL` mapping to
   `z-ai/glm-5.3-flash`; `MODEL_DEFAULT` must never default to Anthropic (owner directive).
5. **Headless permissions** — Claude Code refuses edits headless without
   `--permission-mode bypassPermissions` + allowedTools.
6. **OpenRouter 422 `is_error`** — the CLI sends a non-standard field on tool_result
   turns; llm-proxy sanitize() strips it. **This fix was lost once when the proxy was
   re-deployed from `/tmp` — the proxy source is now canonical in the repo
   (`deploy/m3/llm-proxy/proxy.js`, commit 607b5f6). Lesson: nothing load-bearing
   may live only in a VM's `/tmp`.**
7. **`set -u` vs. spec-composed work packages** — the hermes entrypoint referenced
   `$TASK_PROMPT`, which the lifecycle legitimately omits when the manager composes
   the brief into `behavioral_spec` → unbound-variable exit 1 right after the
   self-report. Fixed on both sides (entrypoint guard + lifecycle coalesce to
   BehavioralSpec, commit 21cac07). **Lesson: M4's direct-spawn gate harness masked
   an integration bug that only the real M2 spawn path exposed.**
8. **§4 gate paper-over** — the acceptance check verified reaped cleanliness but not
   the report's `status`; five runs "passed" §4 while the acceptance task had failed.
   The gate now dies on any non-success report (commit 21cac07). **Lesson: a gate
   must assert the outcome, not the absence of debris.**
9. **ParseAgentIndex both-shapes** — catalog serves `{"agents":[…]}`; the manager
   only accepted a bare array (run 9).
10. **Container git** — the debian-slim manager image lacked `git` for its designed
    clone step (run 8).
11. **M0 gate-2 re-run race** — the cross-VM publish check counts queue depth 2 s
    after publishing, but live consumers drain the durable queue instantly. Proven
    live (`routed:true`, delivered 60→61): test-design race, not a defect.
12. **Estate drift** — vm-hindsight VM 3000 was deleted/replaced mid-project;
    re-provisioned as VMID 2001 via nbd-mount repair (a VG-rename detour broke boot;
    restored via vgcfgrestore). All IPs were reassigned estate-wide; every reference
    (repo, compose, daemon.json, hosts.env) updated in one sweep (1b72825).

## 6. Interim decisions (POC-only, revisited in V2)

| Decision | Rationale |
|---|---|
| `EMBEDDINGS=none` (zero-vector fallback, NaN-safe) | No embeddings provider available in the estate (OpenRouter/agnes verified to serve none) |
| llm-wiki read via the gh keyring token | Distinct fine-grained PAT deferred to V2 key vault |
| Key vault deferred to V2 | Secrets live in gitignored `m3.env` + 0600 key files; POC-only |
| Co-located executor containers on vm-xb | Isolation-level question deferred to V2 |
| One-coder-per-task concurrency | N-parallel + worktrees deferred to V2 |
| Pinned CLIs' bundled deps accepted as residual | Claude Code 2.1.318 and Hermes Agent v0.23.0 bundle their own dependency trees; fixes live upstream — upgrade paths don't clear them (2.1.319 verified identical) |

## 7. Hardening posture (M5 §6, per the M3 TC-M3-06 precedent)

- **Hard gates:** secrets = 0 across all 9 images; fixable HIGH/CRITICAL = 0.
- **Accepted residuals (recorded per-image in `evidence/trivy-poc/`):** unfixed
  base-image OS CVEs, and the pinned third-party CLIs' bundled dependency trees.
- Dependency remediation shipped during M5: amqp091-go v1.13.0, grpc v1.85.0,
  x/net v0.62.0, Go 1.26 toolchain, `apt -y upgrade` in all runtime images.
- Env sweep: no `owner_*`/`github_pat` variables in any running container.

## 8. Locked items (M5 decision log) and Version 2 open items

Locked: VM numbering (final scheme recorded); message-v1 as the versioned contract;
one-coder-per-task; co-located isolation (deferred explicitly); llm-wiki curation
owner = HITL.

**Version 2 open items (from `deploy/m5/decision-log.md`):** multi-flow concurrency
(N-parallel + worktrees); per-task-VM isolation; embeddings provider; long-term
queue shape; key vault / fine-grained PATs; llm-wiki curation automation; upstream
CLI refreshes to clear bundled-dep residuals; topology diagram re-draw (owner).

## 9. Estate state at close

- **vm-ephemeral-agents (<execution-address>, VMID 1000):** RabbitMQ, Temporal + Postgres,
  m2-lifecycle, m2-worker, topic-bridge, llm-proxy; registry-backed images only.
- **vm-hermes-agent (<front-address>, VMID 2000):** m5-gateway (:8080, LAN-published)
  + m5-manager; no GitHub credentials held.
- **vm-hindsight (<memory-address>, VMID 2001):** hindsight-pg (pgvector/pg16) +
  hindsight-api (:8081 LAN-published).
- **vm-docker-registry (<registry-address>, VMID 1001):** private registry with all nine
  `poc/*:1.0.0` images.
- GitHub App `ephemeral-agents-poc` (App ID 5650740, installation 179437771) issues
  per-task tokens; key held 0600 on vm-xb only.

## 10. Conclusion

The POC demonstrates, with signed evidence on real infrastructure, that the
ephemeral-agents architecture is workable: durable orchestration, capability-routed
ephemeral agents performing real repository work, scoped-and-revoked credentials,
cross-task memory, and a human-visible report loop — all under hard gates for
cleanup and credential hygiene. The Version 2 open-items list is the starting
backlog for the next iteration.

*— End of report —*

---

## Addendum — post-close changes (2026-11-08)

Recorded after the POC closed, so the report above remains the accurate
2026-11-07 snapshot:

1. **Agent catalog realized as `agent-roster`.** The machine-readable
   catalog (index.json, 22 eph-* entries) and the executor hydration files
   were committed to the `agent-roster` repository (AGENT ROSTER
   (ephemeral-org)); all pipeline references switched from the planned
   Available Agent List to `agent-roster` (ephemeral-agents commit
   `a1e7dea`; agent-roster commit `f9ec562`). Live smoke verified: a
   capability-only-in-the-new-catalog request routed to
   `ephemeral-org/eph-risk-assessment-analyst`, hydrated fully, and
   returned `status: success`.
2. **The book.** The full thesis → design → proof treatment (14 chapters,
   including a build-from-scratch operator chapter) was written and
   committed under `book/`.
3. **The retrospective.** `POC-Retrospective-and-Insights.md` (effort
   distribution, failure taxonomy, V2 priorities) was committed.

Nothing in this addendum changes any gate result, evidence item, or
performance figure reported above.
