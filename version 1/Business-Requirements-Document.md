# Business Requirements Document — Ephemeral AI Dev Agent System

| | |
|---|---|
| **Status** | Version 1 — POC baseline |
| **Date** | 2026-10-29 |
| **Project** | ephemeral-agents — a standalone proof-of-concept |
| **Companion docs** | `ARCHITECTURE.md` (diagram record) · `Initial-Foundation-Design-Document.md` (design baseline) · `Architectural-Design-Document.md` (buildable design) · `Technical-Feasibility-Analysis.md` (verdict: feasible) · `Technical-Feasibility-POC-Milestones.md` (de-risking plan) |
| **Audience** | Project owner, business stakeholders, and the (junior) developer executing the POC |

---

## 1. Document purpose

This BRD states **what** the business needs the ephemeral-agents POC to demonstrate and **why**, independent of how the technology proves it. It is the business-side counterpart to the design and feasibility documents; it does not restate component design (see the Architectural Design Document) or the de-risking sequence (see the POC Milestones doc).

---

## 2. Background and business problem

The organization's AI development workflow currently depends on human developers to orchestrate every step: reading standards, loading the right agent persona, doing the coding work, and managing the environment. Agent software (Hermes, Claude Code) exists, but no standing arrangement connects them into a *disposable, governed* execution pattern — agents are either run ad hoc on developer machines or would need persistent, always-on infrastructure.

The proposed pattern — **a durable orchestrator driving ephemeral, git-provisioned executor containers** — promises:

- **No standing agent fleet.** Executor containers exist only while work exists; nothing runs idle.
- **Clean state per task.** Each run starts from git-provisioned personas/skills and shared standards; no cross-task contamination.
- **Governed knowledge.** Design standards and best practices come from one shared wiki; long-term memory is a managed store, not scattered chat history.
- **A durable audit trail.** Temporal's workflow history records every task from enqueue to teardown.

Before committing to this pattern at enterprise scale (Version 2), the business requires **evidence, not analysis**, that the pattern works end-to-end on the organization's own substrate (Proxmox + Docker + RabbitMQ + Temporal + GitHub).

---

## 3. Business objectives and success criteria

| # | Objective | Success criterion (measurable) |
|---|---|---|
| BO-1 | Prove the ephemeral-execution pattern is operationally viable on the company substrate | One real human-initiated request completes end-to-end (request → code committed → agents destroyed) with **zero** surviving containers (M5 section 9) |
| BO-2 | Establish RabbitMQ as a durable ingestion boundary usable by future non-Temporal publishers | N messages → exactly N workflows; duplicates and consumer restarts cause no loss and no duplicate work (TP-M1) |
| BO-3 | Prove disposable execution does not leak cost or risk | After repeated runs including hung/crashed containers, no orphan containers remain; teardown is proven idempotent (TP-M2) |
| BO-4 | De-risk the two uncertain executor components before committing budget | A containerized Hermes agent boots with the correct persona/skills from git at startup, statelessly; a headless Claude Code agent makes a real code commit with a scoped credential (TP-M3) |
| BO-5 | Demonstrate governed knowledge hydration | An agent reads **and writes** long-term memory across tasks and consults the correct design standard from the **llm-wiki** git repo (shallow clone, D-wiki-git) on demand (TP-M4) |
| BO-6 | Establish a defensible security posture for agent-held credentials | Credential scans show **no long-lived owner secret reachable from any container**; all repo writes use short-lived, branch-limited tokens (M5 gate) |
| BO-7 | Produce the decision basis for Version 2 | A written Version-2 open-items list exists, and TP-M5 is signed — the feasibility verdict confirmed **by evidence, not analysis** |

**Primary measure of success:** BO-7. The POC succeeds when the business can decide "proceed to Version 2" (or "rework the pattern") on recorded evidence.

---

## 4. Stakeholders and personas

| Stakeholder | Role in this project | Interest |
|---|---|---|
| **Project owner** | Decision authority: settles design items (D-8, D-rmq, D-ttl, D-n1, store-content model), signs gate tests, reviews quarantine | Direction, risk acceptance, budget |
| **Software Engineer (primary user persona)** | Human-in-the-loop: submits requests via Hermes Desktop, receives the outcome | Send a development request and get back an artifact pointer or a clear error — no infrastructure knowledge required |
| **Junior developer (operator persona)** | Executes the POC from the repo alone: install docs → scripts → gate tests | Unambiguous step sequence, deterministic failure routing, evidence trail |
| **Security reviewer (secondary persona)** | Receives the M5 hardening evidence | No owner-secret exposure; least-privilege credentials |
| **Future non-Temporal system teams (downstream)** | Will publish tasks into the RabbitMQ topic in Version 2 | A stable, versioned message contract (`message-v1.json`) |

---

## 5. Scope

### 5.1 In scope (Version 1 POC)
1. One **single flow**: engineer request → gateway → manager → RabbitMQ topic → Temporal → ephemeral executors → hydration → code committed to Project Repo → teardown → human-reported outcome.
2. Three-VM Proxmox chassis (manager VM, execution VM, Hindsight memory VM) with cross-VM network verification. Reference standards live in the **llm-wiki** git repo (D-wiki-git, 2026-11-04: the LLM-Wiki VM and RAG service are removed).
3. The RabbitMQ → Temporal adapter (Go), with idempotency and no-loss guarantees.
4. The container lifecycle manager (Go): spawn / hydrate / teardown, crash-safe reaper, TTL backstop (default 900 s).
5. Executor spikes: containerized Hermes (persona/skills load at boot) and headless Claude Code (real commit, scoped token).
6. Knowledge layer: Hindsight R/W API (Postgres + pgvector) and the **llm-wiki** git repo (github.com/<organization>/llm-wiki, consumed by shallow clone) over design standards; agent hydration from all three sources (llm-wiki, memory, Available Agent List repo).
7. Credential hardening: GitHub App JWT → short-lived installation tokens (`issue-token.sh`), no long-lived secrets in containers.
8. Four failure-injection cases and the end-to-end run as the M5 gate.
9. The Version-2 open-items list documenting everything the POC deliberately did not decide.

### 5.2 Out of scope (Version 1)
- Multi-flow concurrency beyond the single-flow proof (concurrency *semantics* are in scope; concurrent-flow load handling is not).
- Production hardening: HA, multi-tenant isolation, rate limiting, SSO, monitoring dashboards, alerting.
- llm-wiki *content* authoring at scale (curation cadence and ownership are recorded as open items, tracked in the llm-wiki repo's CHANGELOG).
- Any non-Temporal publisher actually publishing to the topic (that is a Version 2 capability; V1 only reserves the boundary).
- Auto-reported or self-triggered flows — the human initiates and receives every outcome.
- Cost accounting / chargeback tooling.

---

## 6. Functional requirements

Requirement IDs are grouped by the milestone that proves them; each FR traces to a business objective and a gate case.

**FR-G Gateway & Manager (M0/M5)**
- **FR-G1 (BO-1):** The system shall accept a development request from the engineer's Hermes Desktop via the Hermes Agent Gateway on the manager VM.
- **FR-G2 (BO-1):** The Manager Agent shall resolve the requesting agent definition by searching the agent index in the Available Agent List repo, and pass the repo location, the markdown folder, and the Behavioral Specification into the spin-up.
- **FR-G3 (BO-1):** The Manager Agent shall publish the task envelope to the RabbitMQ topic and receive the response on the reply path when one is needed (two-way boundary).
- **FR-G4 (BO-7):** The outcome — artifact pointer or error — shall be reported back to the human; no outcome is silently dropped.

**FR-A Adapter (M1)**
- **FR-A1 (BO-2):** The adapter shall start **exactly one** Temporal workflow per valid message, keyed on `task_id` (idempotency key).
- **FR-A2 (BO-2):** Redelivered/duplicate messages shall not start a second workflow.
- **FR-A3 (BO-2):** No message shall be lost across a consumer restart (durable consumer, ack-after-workflow-start).
- **FR-A4 (BO-2, FR-A1..3):** All adapter code shall be Go (owner language mandate).

**FR-L Lifecycle (M2)**
- **FR-L1 (BO-3):** The lifecycle manager shall spawn an ephemeral executor container per task, injecting repo location, markdown folder, Behavioral Specification, per-run credentials, and TTL.
- **FR-L2 (BO-3):** Executor images shall be pre-pulled; no image pull shall occur at activity time.
- **FR-L3 (BO-3):** Teardown shall occur on success, failure, timeout, **and** TTL expiry, and shall be idempotent.
- **FR-L4 (BO-3):** A crash-safe reaper shall be authoritative over container lifetime regardless of orchestrator state; after M runs, zero containers leak.
- **FR-L5 (BO-1):** Concurrent requests shall run concurrent containers, bounded by a team-config hard cap (POC rule of thumb ≤5).

**FR-E Executors (M3)**
- **FR-E1 (BO-4):** A containerized Hermes agent shall boot, load its full persona + skills markdown from the Available Agent List repo at startup, and run statelessly; boot time shall be measured (p50) and acceptable.
- **FR-E2 (BO-4):** A headless Claude Code agent shall clone the target repo, produce a real code change, and commit it using a short-lived, branch-limited token.
- **FR-E3 (BO-4):** The execution model (branch vs. worktree per coder) and the concrete cap value shall be decided and recorded at M3.

**FR-K Knowledge & memory (M4)**
- **FR-K1 (BO-5):** An ephemeral agent shall read **and write** Hindsight long-term memory per session over the network, with read-back demonstrable across tasks.
- **FR-K2 (BO-5):** Reference standards shall be served **wiki-via-git**: an ephemeral agent shall shallow-clone the llm-wiki repo (github.com/<organization>/llm-wiki) at boot and find the relevant design-standard/skill document in the clone (no retrieval API — D-wiki-git).
- **FR-K3 (BO-5):** Design standards content lives in the llm-wiki repo; agent personas/skills in the Available Agent List repo; long-term memory in Hindsight (owner store-content model). Each store has exactly one home.

**FR-S Security (M3/M5)**
- **FR-S1 (BO-6):** Every repo write from an executor shall use a short-lived, branch-limited credential issued per run (GitHub App JWT → installation token); the App key stays on the execution VM at 0600, never in git or containers.
- **FR-S2 (BO-6):** No long-lived owner secret shall be reachable from any executor container; the M5 credential scan is the enforcement gate.
- **FR-S3:** Secrets shall be generated into gitignored `.env` files (openssl rand), never committed and never embedded in compose files.

**FR-T End-to-end (M5)**
- **FR-T1 (BO-1, BO-7):** The full single flow shall execute end-to-end including teardown, leaving zero orphan containers.
- **FR-T2 (BO-3, BO-7):** Four failure-injection cases shall end cleanly and be reported to the human.
- **FR-T3 (BO-7):** The Version-2 open-items list shall be written (listing, not solving, the deferred decisions).

---

## 7. Non-functional requirements

| # | Requirement | Target |
|---|---|---|
| NFR-1 | **Operability** — The installation needs to be clear enough so that a junior developer can complete the full installation and testing without any other help | Step sequence, failure routing, and evidence locations documented; scripts idempotent and re-runnable |
| NFR-2 | **Durability** — task state survives orchestrator/process restarts | Temporal workflow history is the record; adapter acks only after workflow start |
| NFR-3 | **Latency** — executor boot cost is bounded and known | Boot time measured at M3 (B1 p50); TTL default sized from it (900 s) |
| NFR-4 | **Security** — least privilege by construction | Short-lived scoped tokens; secrets at 0600; credential scan gate |
| NFR-5 | **Auditability** — every task traceable | Temporal history + per-milestone evidence files under `deploy/mN/evidence/` |
| NFR-6 | **Recoverability** — failed runs leave nothing behind | Reaper + idempotent teardown (TP-M2 mandatory cases) |
| NFR-7 | **Language standardization** — all Temporal SDK code in Go | Owner mandate 2026-10-24; enforced at each milestone's install check |
| NFR-8 | **Reproducibility** — gates are formal, not informal | A milestone's test procedure must PASS (and be signed) before the next milestone's installation begins |

---

## 8. Business rules

| # | Rule | Source |
|---|---|---|
| BR-1 | The gate rule: no milestone installation begins until the previous milestone's test procedure is signed | POC plan |
| BR-2 | Executor container counts are needs-based, never fixed; parallel requests get parallel containers under a hard config cap | Owner, 2026-10-25 |
| BR-3 | Only the manager writes `ttl_s`; the lifecycle default (900 s) applies when absent; the reaper is authoritative in all cases | Owner, 2026-10-29 |
| BR-4 | RabbitMQ is retained deliberately as the V2 ingestion boundary; the adapter's idempotency machinery is the accepted cost of owning that boundary | Owner, 2026-10-29 |
| BR-5 | The POC shares no architecture with any production pipeline | README scope note |
| BR-6 | VM names are deploy-time choices per project; the POC labels are discussion devices only | D-8, 2026-10-25 |
| BR-7 | Scripts of unverified provenance stay quarantined (gitignored `_quarantine/`) pending owner review | .gitignore / README |

---

## 9. Assumptions, constraints, dependencies

**Assumptions** (stated, not verified — verified by M0 where applicable):
- Proxmox 9.x hosts KVM VMs running Docker; the three VMs are mutually network-reachable and reach GitHub.
- Temporal, RabbitMQ, and Postgres/pgvector run as containers on the same substrate.
- The Manager Agent is a persistent, containerized agent.
- The engineer's desktop can reach the gateway over the company network.

**Constraints:**
- All Temporal SDK code in Go (BR/owner mandate).
- Single-flow scope; the human initiates and receives everything.
- POC budget: the ≤5 container cap is a rule of thumb sized to VM configuration, not an absolute.
- The POC must remain standalone (no production-pipeline architecture shared in).

**Dependencies:**
- GitHub (Available Agent List + Project Repo) reachable from executors.
- A GitHub App with permission to mint short-lived installation tokens (M3 asset).
- Hermes agent runtime and Claude Code headless licensing/availability inside containers (the two To-verify items).

---

## 10. Business risks

| # | Risk | L | I | Mitigation / owner |
|---|---|---|---|---|
| BR-R1 | The pattern proves unbuildable in one or both executor spikes | M | H | Spikes are deliberately early (M3 = point of last cheap exit); fallback designs identified (hydration sidecar; alternative coder runtime) |
| BR-R2 | Integration glue (adapter, lifecycle) costs more than estimated | M | M | Bounded scope; both are the *named* custom pieces; gates T/V cases make overrun visible early |
| BR-R3 | Credential misuse by ephemeral agents | L | H | FR-S1..3 + M5 credential scan gate |
| BR-R4 | Knowledge stores drift (stale standards, uncurated llm-wiki repo) → agents produce off-standard work | M | M | Curation cadence is an explicit open item, tracked in the llm-wiki repo's CHANGELOG; git versioning is built in (R7) |
| BR-R5 | POC succeeds but the organization defers V2, losing momentum | M | M | BO-7 forces a written decision basis; open-items list keeps V2 cheap to start |
| BR-R6 | Junior-operator execution stalls on environment drift | L | M | Idempotent scripts, `--check-only`, failure routing to doc sections (NFR-1) |

---

## 11. Acceptance criteria (POC-level)

The POC is accepted when **all** hold (mirrors M5 section 9):
1. One real request ran end-to-end including teardown — zero surviving POC containers.
2. All four failure injections ended clean and were reported to the human.
3. Security scans pass; no long-lived owner secret reachable from any container.
4. The Version-2 open items are listed (not solved).
5. TP-M5 is signed.

On acceptance, the feasibility verdict ("technically feasible — no fundamental blocker") is confirmed **by evidence, not analysis**, and the business decides on Version 2 with the open-items list in hand.

---

## 12. Glossary

| Term | Meaning |
|---|---|
| **Ephemeral executor** | A short-lived container performing one task, destroyed on completion |
| **Manager Agent** | Persistent agent on the manager VM; resolves agent definitions and publishes tasks |
| **Available Agent List** | Git repo holding agent personas, skills, tools, markdown files, and the Behavioral Specification |
| **Behavioral Specification** | The behavior contract passed into each agent spin-up |
| **Hindsight** | Long-term memory store (Postgres + pgvector) with a session R/W API |
| **llm-wiki** | Shared git repo (github.com/<organization>/llm-wiki) of design standards and dev/business best practices, consumed by shallow clone (D-wiki-git) |
| **Topic-bridge adapter** | Go service: RabbitMQ consumer → Temporal workflow start, exactly-once effect |
| **Lifecycle manager** | Go service owning spawn/hydrate/teardown of executor containers + reaper |
| **Gate (TP-Mn)** | The milestone test procedure that must PASS and be signed before the next milestone's install begins |
| **`message-v1.json`** | The versioned task-envelope contract between manager, topic, and adapter |
| **V2 ingestion boundary** | The RabbitMQ topic, kept so non-Temporal systems can publish tasks in Version 2 |

---

*This BRD is business-side traceability for the Version 1 doc set. Requirement IDs trace to gate cases via the POC Milestones document; design answers live in the Architectural Design Document.*
