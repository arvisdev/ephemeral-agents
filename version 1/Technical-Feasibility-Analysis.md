# Technical Feasibility Analysis — Ephemeral AI Dev Agent System (Version 1)

| | |
|---|---|
| **Status** | Desk-top feasibility assessment (no components built or run yet) |
| **Date** | 2026-10-24 |
| **Scope** | The **Version 1** architecture as recorded in `ARCHITECTURE.md` and `Initial-Foundation-Design-Document.md` |
| **Method** | Component-by-component and flow-level assessment against proven technology. Each item is rated **Proven / Feasible / Needs-build / To-verify** with a short rationale. Nothing here claims to have been exercised — it is a desk-top assessment to guide a proof-of-concept. |

**Rating scale**

- **Proven** — off-the-shelf, widely used, low risk.
- **Feasible** — clearly doable with known technology; effort, not uncertainty.
- **Needs-build** — no off-the-shelf piece; a custom glue/adapter must be written.
- **To-verify** — plausible but requires a small spike before we commit to it.

---

## 1. System under analysis (Version 1, one flow)

A human **Software Engineer** (Hermes Desktop, outside the boundary) requests work → **Hermes Agent Gateway + Manager Agent** (VM-Xa, persistent, in Proxmox) → **RabbitMQ (Topic)** (VM-Xb) → **Temporal** enqueues/polls → **"Executes Activity"** spins up **ephemeral Docker containers as needed** (a Hermes Agent for agent-type work, a Claude Code Agent when coding occurs — the "(N+1)" label means variable count, not a fixed two; concurrent requests run concurrent containers, team-config capped, POC rule of thumb ≤5 per VM sizing) → they hydrate from **LLM-Wiki (VM-2)**, the **Available Agent List** repo, and **Hindsight long-term memory (VM-3)** → the Claude Code agent **writes source code to the Project Repo** → the agents and containers are **destroyed**.

The single load-bearing idea: **a durable orchestrator (Temporal) drives ephemeral, git-provisioned executor containers**, with knowledge split across a shared RAG wiki, a vector memory, and a git-hosted agent-catalog.

## 2. Component feasibility

| Component | Rating | Rationale / notes |
|---|---|---|
| Proxmox 9.x + KVM VMs + Docker host | **Proven** | Already the user's running environment; Docker on KVM VMs is standard. |
| **VM-Xa** — Hermes Agent Gateway + Hermes Agent (Docker) + Manager Agent | **Feasible** | A gateway + a long-lived manager agent in a container is routine. *(To-verify: which runtime hosts the Manager Agent.)* |
| **RabbitMQ (Topic)** | **Proven** | Standard pub/sub; topic exchange maps cleanly to "manager publishes, queue consumes." |
| **Temporal Server + Worker** | **Proven** | Temporal is a mature durable-orchestration platform; durable activities, retries, timeouts are exactly what an ephemeral-executor flow needs. |
| **RabbitMQ → Temporal bridge** | **Needs-build** | Temporal does not natively consume RabbitMQ. A small **adapter service** (RabbitMQ consumer → Temporal workflow start) must be written. This is a real, bounded integration point. |
| **"Executes Activity" → spawn ephemeral containers** | **Feasible / Needs-build** | A Temporal activity that `docker run`s a container is doable; the **lifecycle manager** (image pull, env injection, teardown, crash recovery) is the custom part. |
| **Hermes Agent (Ephemeral)** | **To-verify** | Containerized Hermes is plausible. **Spike required:** confirm a container can boot Hermes and load persona + skills (markdown) from a git repo **at startup**, quickly, statelessly. |
| **Claude Code Agent (N+1)** | **To-verify** | Headless Claude Code in a container is plausible. **Spike required:** repo checkout + toolchain, scoped git/API credentials, and reliable "write source code to Project Repo." **Risk:** N parallel coders writing the same repo → conflicts. |
| **Hindsight — Long-Term Memory (VM-3)** | **Feasible** | Postgres + pgvector memory is proven. **To-verify:** expose an **API** that ephemeral agents can **R/W** at session time. |
| **LLM-Wiki (VM-2)** | **Feasible** | Shared RAG / standards store is proven. **To-verify:** curation/indexing pipeline that keeps design standards + best practices current. |
| **GitHub repos (Available Agent List, Project Repo)** | **Proven** | Git + GHA/API. **Security:** ephemeral agents need **scoped/ephemeral** credentials (short-lived PAT / GITHUB_TOKEN, branch-limited) — not the owner's long-lived token. |
| **Ephemeral teardown (destroy agent + container)** | **Feasible / Needs-build** | Docker lifecycle + Temporal on-complete/cleanup; needs **reliable teardown + error handling** so failed runs don't leak containers. |
| **Software Engineer / Hermes Desktop (external)** | **Proven** | The user's desktop is the initiator/report sink. |

**Net read:** every component is at least **Feasible**; none is a fundamental blocker. Two items are **To-verify** (Hermes ephemeral persona-load; headless Claude Code container), one is a **Needs-build** glue (RabbitMQ→Temporal adapter + container-lifecycle manager).

## 3. Flow / integration feasibility (step by step)

1. **Engineer → Gateway** — trivial (proven desktop ↔ VM network).
2. **Manager ↔ RabbitMQ (Topic)** — proven; the two-way variant is fine (topic publish + a response queue / correlation).
3. **Topic → Temporal** — **the key glue.** Build the RabbitMQ consumer that starts a Temporal workflow per message. Bounded, well-understood.
4. **Temporal polls queue → Executes Activity** — proven Temporal semantics.
5. **Spawn ephemeral containers** — **the core custom piece.** Feasible; the lifecycle manager + image pre-build + deterministic teardown is where most of the POC effort sits.
6. **Hydrate** — LLM-Wiki (RAG), Available Agent List (git pull markdown), Hindsight (R/W API). Feasible; depends on the two To-verify items and a Hindsight API.
7. **Work + write to Project Repo** — feasible; **credential scoping + concurrent-writer handling** are the open security/concurrency concerns.
8. **Teardown** — feasible; needs a reliable cleanup path (including on crash/failure).

## 4. Key technical risks (risk register)

| # | Risk | L | I | Mitigation |
|---|---|---|---|---|
| R1 | Ephemeral container **lifecycle** (leaks, crash recovery, clean teardown) | M | H | Temporal durable activities + timeouts + a lifecycle-manager with idempotent teardown; container TTL/reaper. |
| R2 | **RabbitMQ→Temporal bridge** correctness/loss | M | M | Durable consumer (ack after workflow start); at-least-once + idempotent workflow start. |
| R3 | **N+1 concurrent coders** writing one repo → conflicts | M | M | One coder per task (branch/worktree per coder); or serialize repo writes; git worktrees. |
| R4 | **Credential exposure** to ephemeral agents | M | H | Short-lived / scoped tokens (branch-limited, least-privilege), secrets injected per-run, no long-lived owner token in containers. |
| R5 | **Hermes persona/skill load at boot** is slow or not supported | L | M | Spike first; fall back to sidecar that hydrates the container before the agent starts. |
| R6 | **Hindsight R/W API** not available for agent sessions | L | M | Define the memory contract early; use a thin API wrapper; pgvector is the substrate. |
| R7 | **LLM-Wiki curation** (standards go stale) | M | M | Index pipeline + owner curation cadence; version the standards. |
| R8 | **Cross-VM service discovery** (VM-Xb agents must reach VM-2, VM-3, cloud) | L | M | Flat L2/overlay network or DNS/service-catalog; fixed service endpoints per VM. |
| R9 | **Proxmox/VM naming** (VM-2/3 vs VM-Xa/Xb) unsettled | L | L | **Resolved by owner 2026-10-25:** the labels are POC discussion devices, not commitments; per-project naming is a deploy-time choice. POC records the decision (M0 §6) and moves on. |

*(L = likelihood, I = impact.)*

## 5. Items to verify before committing (spikes)

1. **Hermes ephemeral:** boot a Hermes container; load a persona + skills markdown from a git repo at startup; measure boot time and correctness.
2. **Claude Code headless:** run Claude Code in a container against a cloned repo; confirm it can make + commit code with a scoped token.
3. **RabbitMQ→Temporal adapter:** prove one message starts exactly one Temporal workflow (idempotency + no loss).
4. **Hindsight R/W API:** confirm ephemeral agents can read + write memory per session over the network.
5. **LLM-Wiki ingest:** confirm design standards can be indexed and retrieved by agents.
6. **Concurrency model:** decide **one coder per task vs. N parallel coders** and validate the repo-write strategy.

## 6. Feasibility verdict

**Technically feasible — no fundamental blocker.** The architecture composes well-understood, proven pieces (durable orchestrator, ephemeral executors, RAG + vector memory, git-hosted agent catalog, scoped git credentials) into a pattern that exists in industry. The effort concentrates in **integration glue** (RabbitMQ→Temporal adapter, the ephemeral-container lifecycle manager) and in **validating two To-verify components** (Hermes boot-time persona load; headless Claude Code container).

**Conditions for the verdict to hold:**
- The **RabbitMQ→Temporal** bridge and the **container lifecycle manager** are built and de-risked (R1, R2).
- The two **To-verify** components pass their spikes (§5, items 1–2).
- **Credential scoping** and a **concurrency model** for repo writes are decided (R3, R4).
- **Hindsight** exposes a usable R/W API for agent sessions (R6).

If §5 spikes pass, the Version 1 POC is buildable with standard tooling.

## 7. De-risking build order (summary)

Full plan in `Technical-Feasibility-POC-Milestones.md`. Order by what de-risks the most:

1. **M0** Proxmox + VMs + Docker + RabbitMQ + Temporal standing up (the chassis).
2. **M1** RabbitMQ→Temporal adapter (proves the decoupling path).
3. **M2** Container lifecycle manager (spawn / hydrate / teardown) — the core custom piece.
4. **M3** The two executor spikes (Hermes ephemeral, Claude Code headless) wired into the lifecycle.
5. **M4** Memory + knowledge (Hindsight API, LLM-Wiki) wired in.
6. **M5** End-to-end single flow + teardown; credential scoping + concurrency model locked.

## 8. Assumptions (stated, not verified)

- Proxmox 9.x hosts KVM VMs running Docker; VM-Xa/Xb/2/3 are network-reachable to each other and to GitHub.
- Temporal + RabbitMQ + Postgres/pgvector are all provisioned as containers/VMs on the same substrate.
- The Manager Agent is a persistent, containerized agent (runtime to be confirmed).
- "N+1" means one Hermes agent + N coder agents; the concurrency model is open.
- This is a **single-flow** POC; multi-flow concurrency is out of scope for Version 1.

## 9. Open design items (carried from Version 1)

The Version 1 foundation doc intentionally leaves these open; feasibility does not depend on resolving them now, but each gates a later milestone: RabbitMQ↔Temporal wiring detail (R2), LLM-Wiki curation (R7), ephemeral isolation level (per-task VM vs. co-located container), N+1 semantics (R3), and topic/queue message shapes. (VM naming, R9, was settled by the owner on 2026-10-25 — the labels are POC discussion devices, not commitments; M0 records the decision.)

---

*Companion files: `ARCHITECTURE.md`, `Initial-Foundation-Design-Document.md` (the design record); `Technical-Feasibility-POC-Milestones.md` (the de-risking plan).*
