# Architectural Design Document — Ephemeral AI Dev Agent System

| | |
|---|---|
| **Status** | Version 1 design — formalized from the owner-approved diagram + feasibility analysis |
| **Date** | 2026-10-29 |
| **Source docs** | `version 1/ARCHITECTURE.md`, `Initial-Foundation-Design-Document.md`, `Technical-Feasibility-Analysis.md`, `Technical-Feasibility-POC-Milestones.md` |
| **Scope** | Single-flow POC. This document turns the diagram record + feasibility verdict into a buildable design: components, contracts, state machines, deployment topology, failure modes. It does not repeat the diagram transcription. |

---

## 1. Purpose and context

A human Software Engineer (human-in-the-loop, Hermes Desktop, outside the company boundary) submits a development request. A durable orchestrator (Temporal) drives short-lived, git-provisioned executor containers that hydrate from shared knowledge sources, perform one unit of development work, and are destroyed with nothing left behind. The outcome — artifact pointer or error — returns to the human.

**Load-bearing design decision:** *a durable orchestrator drives ephemeral, git-provisioned executor containers*, with knowledge split across the git-hosted **llm-wiki** reference repo, a vector long-term memory, and a git-hosted agent catalog (D-wiki-git, 2026-11-04: the LLM-Wiki RAG service was removed; the wiki is a plain git repo consumed by clone).

**Hard constraints (owner-settled):**
1. All Temporal SDK code is **Golang** (workflows, workers, activities, adapters, services, CLI samples).
2. **RabbitMQ stays** as the V2 ingestion boundary — non-Temporal systems will publish tasks to the topic alongside the manager; the adapter's idempotency/no-loss machinery is the accepted cost.
3. Concurrency semantics: needs-based container counts, parallel containers for parallel requests, hard team-config cap (POC rule of thumb ≤5, VM-size dependent). Execution model (branch vs. worktree) and cap value are locked at M3.
4. VM labels (`vm-xa`, `vm-xb`, `vm-hindsight`) are POC-local; per-project naming is a deploy-time choice (D-8). (D-wiki-git, 2026-11-04: `vm-llm-wiki` / VM-2 is dropped — 3-VM topology.)
5. Hybrid TTL: `ttl_s` optional in `message-v1.json`, written only by the manager (sized from M3 B1 boot-time p50); absent → lifecycle default `POC_TTL_DEFAULT=900s`; the reaper is authoritative either way.

## 2. System context

```
                    ┌──────────── On-Prem Company Boundary ────────────┐
 Engineer (Hermes   │  ┌── Proxmox 9.x ──────────────────────────┐     │
 Desktop, external)─┼─►│ vm-xa: Gateway + Manager Agent          │     │
                    │  └──────────────────────────────────────────┘    │
                    │  ┌── vm-xb ─────────────────────────────────┐    │
                    │  │ RabbitMQ (topic) · Temporal Server+Worker│    │
                    │  │ [ephemeral: Hermes agent, Claude Code]   │    │
                    │  └───────────────────────────────────────────┘   │
                    │  vm-hindsight (pgvector memory)                  │
                    └───────────────────────────────────────────────────┘
                    Cloud: GitHub — Available Agent List repo · llm-wiki repo (github.com/<organization>/llm-wiki) · Project Repo
```

- **Persistent:** Gateway + Manager (vm-xa); queue + Temporal (vm-xb); memory VM.
- **Reference knowledge:** the **llm-wiki** git repo on GitHub — no VM, no service; consumed by shallow clone.
- **Ephemeral:** executor containers on vm-xb, spawned per task, destroyed on completion.
- The engineer is the only external actor; the flow is human-initiated and human-reported.

## 3. Components and responsibilities

| Component | VM | Role | Maturity |
|---|---|---|---|
| **Hermes Agent Gateway** | vm-xa | Accepts engineer requests; front door for the flow | Build (Go) |
| **Manager Agent** | vm-xa | Resolves the agent definition from the Available Agent List index (Note 1), publishes the task envelope to the topic, receives the response (two-way) | Build (Go) |
| **RabbitMQ topic exchange** | vm-xb | Decouples manager from execution; the V2 ingestion boundary | Proven |
| **Topic-bridge adapter** | vm-xb | RabbitMQ consumer → Temporal workflow start; exactly-once-effect per message | Build (Go, amqp091-go + Temporal SDK) |
| **Temporal Server + Worker** | vm-xb | Durable orchestration: task queue, activities, retries, timeouts | Proven |
| **Container lifecycle manager** | vm-xb | Spawn/hydrate/teardown of ephemeral containers via `docker run`; crash-safe reaper; idempotent teardown | Build (Go) — the core custom piece |
| **Hermes ephemeral agent** | vm-xb (container) | Agent-type work; boots persona + skills from git at startup, stateless | To-verify (M3a spike) |
| **Claude Code agent (N+1)** | vm-xb (container) | Coding; clones repo, commits with a scoped token | To-verify (M3b spike) |
| **Hindsight** | vm-hindsight | Long-term memory (Postgres + pgvector) behind a R/W session API | Build (API) |
| **GitHub repos** | cloud | Available Agent List (personas, skills, behavioral spec) + **llm-wiki** (reference standards, D-wiki-git) + Project Repo | Proven |

## 4. Data and control flow

1. **Request in.** Engineer → Gateway (vm-xa).
2. **Resolve.** Manager Agent looks up the agent index in the Available Agent List repo; selects persona/skills/tool set; obtains repo location + markdown folder + Behavioral Specification.
3. **Publish.** Manager → topic exchange `agent.tasks` with routing key `agent.tasks.<capability>` (binding pattern `agent.tasks.#`; DLQ `agent.tasks.dlq`; responses on `agent.tasks.response`). Two-way: the response returns on the same `task_id` when one is needed (D-msg-v1).
4. **Bridge.** Topic-bridge consumes, validates `message-v1.json`, starts exactly one Temporal workflow per message (idempotency key = `task_id`; ack-after-workflow-start → no loss, no duplicates' second workflow).
5. **Orchestrate.** Temporal workflow runs the "execute activity," which calls the lifecycle manager.
6. **Spawn + hydrate.** Lifecycle manager `docker run`s the needed executor containers (pre-pulled images, no pull at activity time), injecting: repo location + markdown folder, Behavioral Specification, per-run scoped credentials, TTL. Executors hydrate:
   - Available Agent List repo → full persona/skill markdowns;
   - llm-wiki repo → shallow git clone → design standards/tools/skills (D-wiki-git);
   - Hindsight API → memory read; memory write on completion.
7. **Work.** Claude Code agent makes code changes and commits to the Project Repo (branch/worktree per coder — locked at M3).
8. **Teardown.** On success, failure, timeout, or TTL expiry the lifecycle manager destroys the containers — idempotently, including after a manager/worker crash (reaper backstop). Zero surviving containers.
9. **Report.** Result returns through the manager to the engineer (human-reported; no auto-flow).

## 5. Contracts

### 5.1 Message envelope — `message-v1.json`
The inter-zone contract: [`schemas/message-v1.json`](../schemas/message-v1.json) (written by the M1 delivery plan, WP-1.1). Required fields: `task_id` (unique, **the idempotency key** — the Temporal workflow ID is `task_id`), `agent_ref` (repo location + folder with the agent's markdown), `behavioral_spec`, `repo_target`, `requester`. One optional field: `ttl_s` (integer seconds; manager is the only writer; absent ⇒ M2 applies `POC_TTL_DEFAULT`, the reaper is authoritative either way). There is **no `correlation_id`**: `task_id` is the single identifier — the two-way response path correlates on `task_id` (M2 response publisher → M5 manager consume). Versioned at the top level (`$schema`) so V2 changes don't break the bridge.

### 5.2 Service APIs
- **Hindsight R/W API** (`services/hindsight-api`): session-scoped read/write of memory records; pgvector substrate; contract fixed early (R6) so the executor entrypoints are stable.
- **Lifecycle API** (`services/lifecycle`): spawn(task) / status(id) / teardown(id); teardown is idempotent; the reaper sweeps TTL-expired and orphaned containers independent of Temporal.

### 5.3 Git interface
Executors receive: repo URL, branch/worktree name, a **short-lived, branch-limited token** issued by `issue-token.sh` (GitHub App JWT → installation access token, run on vm-xb; app key at `/opt/poc/secrets/app-key.pem`, 0600, never in git or containers).

## 6. Lifecycle state machine (per executor container)

```
SPAWNED → PULLING(implicit; images pre-pulled) → RUNNING → (SUCCESS|FAILED|TIMED_OUT) → TEARDOWN → GONE
                    └──────────── TTL_EXPIRED ─────────────────────────────────────┘
   crash of anything at any point → REAPER sweep → idempotent teardown → GONE
```

Invariants: teardown is safe to re-run; after M runs, zero leaked containers (TP-M2 mandatory case); no orphan survives a worker crash (reaper is authoritative, R1).

## 7. Deployment topology

- **Proxmox 9.x host** provisions 3 KVM VMs (`deploy/m0/hosts.env`, netplan record-only unless `--apply-netplan`).
- vm-xa: gateway + manager containers (M5 compose).
- vm-xb: rabbitmq + temporal (+ their Postgres) compose (M0); topic-bridge (M1); lifecycle services (M2); executor images pre-loaded (M3); gateway/manager deploy (M5).
- vm-hindsight: memory compose + API container (M4). Reference standards live in the **llm-wiki** git repo (D-wiki-git) — no wiki VM, no wiki service.
- Flat L2/overlay network + fixed DNS endpoints per VM (R8, verified by the M0 network matrix).
- Install path: `deploy/mN/install.sh` per milestone (idempotent, `--check-only`, gate tests → `evidence/`), gated by signed TP-Mn; `hosts.env`/`.env` gitignored, secrets generated via `openssl rand -hex 16`, never in compose files or git.

## 8. Failure modes and mitigations

| Failure | Design answer |
|---|---|
| Leaked/hung container (R1) | Lifecycle manager idempotent teardown + TTL reaper + Temporal activity timeouts; M2 gate proves the crash path. |
| Message loss / duplicate workflow (R2) | Durable consumer, ack after workflow start, idempotency key on `task_id`; M1 gate TC-M1-01…06. |
| N coders on one repo (R3) | Branch/worktree per coder (locked M3); cap in team config. |
| Credential exposure (R4) | Short-lived branch-limited tokens per run; no long-lived owner secret in any container; M5 credential scan is the gate. |
| Slow/unsupported Hermes boot persona load (R5) | M3a spike first; fallback = hydration sidecar fills the container before the agent starts. TTL sized from measured boot p50. |
| Hindsight API gap (R6) | Contract defined early; thin Go wrapper over pgvector. |
| Stale standards (R7) | Content lives in the llm-wiki git repo (D-wiki-git) — versioned by git; serving-layer risk retired (no ingest/reindex). Owner curation cadence tracked in the repo CHANGELOG; curation ownership remains open. |
| Cross-VM reachability (R8) | M0 network verification matrix as the gate. |
| Stage failure at M5 | Failure injections F1–F4 are mandatory cases; each stage names its owning milestone for re-run. |

## 9. Milestone mapping (design → proof)

| Milestone | Proves the design element | Gate |
|---|---|---|
| M0 – Infrastructure Foundation | section 2, section 7 — VMs, Docker, topic, Temporal, Go SDK smoke, network matrix | TP-M0 (8 cases) |
| M1 – Message-to-Workflow Bridge | section 4.4, section 5.1 — exactly-one-workflow, idempotent, no loss | TP-M1 (6 cases) |
| M2 – Container Spawn & Reap System | section 6 — spawn/timeout/reap state machine, crash-safe, no-pull gate | TP-M2 (6 cases) |
| M3 – Agent Executor Proofs | section 3 executors, section 5.3 — Hermes boot-time persona load (point of last cheap exit); headless Claude Code real commit; lock execution model + cap | TP-M3 (7 cases) |
| M4 – Agent Memory & Knowledge Access | section 5.2 — Hindsight R/W, wiki-via-git hydration (llm-wiki clone), git hydration, cross-task read-back | TP-M4 (7 cases) |
| M5 – Full System Integration Test | section 4 end-to-end, four failure injections, credential hardening, V2 open-items list | TP-M5 |

**POC exit condition (M5 section 9):** E2E passed with zero surviving POC containers; four failure injections clean and human-reported; no long-lived owner secret reachable from any container; Version-2 open items listed; TP-M5 signed — confirming the feasibility verdict *by evidence, not by analysis*.

## 10. Open items (explicitly deferred, not pre-decided here)

1. RabbitMQ↔Temporal wiring detail + message-shape finalization (R2/M1, `message-v1.json`).
2. llm-wiki curation ownership and promotion rules (R7/M4) — tracked in the llm-wiki repo's CHANGELOG.md.
3. Ephemeral isolation level — co-located containers on vm-xb vs. per-task VMs (M3/M5).
4. N+1 execution model (branch vs. worktree) and the concrete cap value (M3).
5. llm-wiki ↔ Hindsight ↔ GitHub unlabeled flows from the source diagram (whether learnings promote into repos/llm-wiki) — resolve at M4.
6. **Wiki hydration/retrieval shape (D-wiki-git → resolved by D-hydrate, 2026-11-04)** — RESOLVED: full shallow clone (`git clone --depth 1`) of `github.com/<organization>/llm-wiki` at executor boot is the standing decision; revisit trigger recorded in D-hydrate (clone time > 20% of TTL headroom across 3 runs, or repo > 50 MB) — then move to a curated index / path-scoped fetch (V2 candidate).

These resolutions (or explicit deferrals) form the **Version 2 open-items list** required at M5 exit.

## 11. Decision log (carried)

| # | Decision | Date |
|---|---|---|
| D-wiki-git | **The LLM-Wiki RAG service is removed entirely; the wiki is a plain git repo** (owner, 2026-11-04): the owner-curated `llm-wiki` git repository (private; access requires authorization) (private; scaffolded with design-standards/, dev-best-practices/, business-best-practices/, tools-and-skills/, CHANGELOG.md, README). Consumers shallow-clone it via git — no retrieval API, no pgvector serving layer, no ingest pipeline. **vm-llm-wiki / VM-2 / VMID 902 is dropped** — the POC is a 3-VM topology (vm-xa, vm-xb, vm-hindsight). Hindsight unchanged. D-wiki-truth is now trivially true (the content repo IS the store); D-spec-source unchanged; load-bearing standards still pinned byte-exact into behavioral-spec.md. Hydration shape RESOLVED 2026-11-04 by D-hydrate (full shallow clone, revisit trigger recorded there). Curation cadence/ownership tracked in the repo's CHANGELOG. Supersedes the RAG-service aspects of D-stores, D-wiki-truth, D-mp-shared, and section 5.2's llmwiki-api. | 2026-11-04 |
| D-hydrate | **llm-wiki hydration shape** (owner, 2026-11-04): executors consume the llm-wiki by a **full shallow clone at boot** (`git clone --depth 1`), navigated via the repo's own README/directory layout — no retrieval aid in V1. **Revisit trigger:** if clone time exceeds 20% of TTL headroom across TC-M4-05's 3 runs, or the repo exceeds 50 MB, move to a curated index (`INDEX.md`) with path-scoped fetch (V2 candidate). Content stays the repo's own truth (D-wiki-truth). | 2026-11-04 |
| D-8 | VM labels are POC discussion devices; per-project naming at deploy time | 2026-10-25 |
| D-lang | All Temporal SDK code is Golang | 2026-10-24 |
| D-rmq | RabbitMQ kept deliberately as the V2 ingestion boundary | 2026-10-29 |
| D-ttl | Hybrid TTL: manager-written optional `ttl_s`, lifecycle default 900 s, reaper authoritative | 2026-10-29 |
| D-msg-v1 | Message contract resolved: `schemas/message-v1.json` is authoritative. Fields: `task_id` (THE idempotency key / workflow ID), `agent_ref`, `behavioral_spec`, `repo_target`, `requester`, optional `ttl_s` (manager-only writer). **No `message_id`, no `correlation_id`** — `task_id` is the single identifier; the response path correlates on `task_id`. section 5.1 rewritten to match. | 2026-10-30 |
| D-catalog | Available Agent List lives in **its own GitHub repo** (company-boundary, outside the POC), owner-curated (R7), consumed by M3 hydration and the M5 manager via a dedicated read token. Layout contract (index.json shape + per-agent folder) recorded in `Available-Agent-List-Repository.md`; working example at `deploy/m5/agent-list-example/`. | 2026-10-30 |
| D-mp-shared | **The ephemeral-agent design is per-project; Hindsight and LLM-Wiki are shared company resources across projects** (owner, 2026-10-30). Beyond the POC, each project provisions its **own VM-Xa + VM-Xb pair** (own naming + IPs, per D-8) and gains access to the **same** Hindsight VM and LLM-Wiki VM. POC consequence: the shared VMs' UFW allow-lists are per-project (add each project pair's subnet — `UFW_POC_CIDR` in the m4 script is the POC's single-project instance of that rule). V2 open items this raises: per-project memory scoping in Hindsight, shared-VM capacity planning, firewall allow-list governance. | 2026-10-30 |
| D-proj-id | **Every project has a unique project-id, which identifies its provisioned space in Hindsight** (owner, 2026-10-30). All Hindsight writes/reads carry the project-id; project A's agents can never see project B's memory. Resolves the "per-project memory scoping" V2 open item at decision level — V2 adds the project-id dimension to Hindsight's scope model (the POC's single-project session/task scoping is the seed). | 2026-10-30 |
| D-wiki-truth | **The LLM-Wiki is the source of truth for reference documentation** — both per-project reference material and company/enterprise documentation shared across all projects (company design specifications and the like). Content taxonomy: a per-project area (keyed by project-id) + a shared enterprise area. | 2026-10-30 |
| D-repo-truth | **Each project has its own project repository (GitHub/ADO) as the source of truth for source code and project documentation** — everything versionable that belongs to the project's development deliverables. **Boundary rule (owner, 2026-10-30):** development- and business-related items relevant to the deliverables → the project repo; **reference material → the LLM-Wiki**. This is the disambiguation agents apply when deciding where a document lives. | 2026-10-30 |
| D-repo-ops | **Project-repo setup and lifecycle sit OUTSIDE the ephemeral-agents project** (owner, 2026-10-30). Repositories (GitHub/ADO) are provisioned and managed by a well-defined DevOps process (agile/scrum and the like) that is not part of this architecture; ephemeral-agents' only relationship to them is **read/write access** via short-lived, branch-limited per-run tokens (FR-S1/S2). The POC reflects this: `repo_target` arrives in the task message; nothing in M0–M5 creates or administers project repos. | 2026-10-30 |
| D-spec-source | **Agent specifications persist ONLY in the Available Agent List repo** (D-catalog) — never duplicated into the LLM-Wiki. The catalog is already the cross-project uniformity mechanism: every project's manager and executors clone the same source at runtime. The LLM-Wiki's shared enterprise area holds **agent-design standards** (templates, persona/skills/tools norms, guardrail guidance) that catalog curation follows — standards in the wiki, operational specs in the catalog; two artifacts, one source of truth each. | 2026-10-30 |
| D-routing | Topic namespace resolved: exchange `agent.tasks`, routing key `agent.tasks.<capability>`, binding `agent.tasks.#`, DLQ `agent.tasks.dlq`, responses `agent.tasks.response`. The ADD's earlier `tasks.dev.*` was a pre-decision placeholder — retired, not reconciled; nothing implements it. `agent.tasks.#` serves the D-rmq ingestion-boundary rationale (V2 publishers land in the same durable queue with no re-binding). section 4 flow rewritten to match. | 2026-10-30 |
| D-n1 | Needs-based counts, parallel for parallel requests, team-config hard cap (≤5 POC thumb) | 2026-10-25 |
| D-stores | Hindsight = long-term memory (R/W); LLM-Wiki = standards/best practices; Available Agent List repo = everything agents load | 2026-10-24 |

---

*This design document is the buildable reading of the Version 1 doc set. Where it goes beyond the diagram record (section 5 contracts, section 6 state machine, section 8 failure table), it is new, clearly-marked design content per the baseline rule — not silently mixed into the foundation record.*
