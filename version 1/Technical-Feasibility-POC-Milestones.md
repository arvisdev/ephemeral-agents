# POC Milestones & De-risking Plan — Ephemeral AI Dev Agent System (Version 1)

| | |
|---|---|
| **Status** | De-risking plan (companion to `Technical-Feasibility-Analysis.md`) |
| **Date** | 2026-10-24 |
| **Purpose** | Sequenced milestones that de-risk the Version 1 architecture in dependency order. Each milestone has a **goal**, **done-when (success criteria)**, and the **risk(s) it retires**. Not a build spec — it is the order to *prove feasibility*, not to build the full system. |

Principle: **prove the risky parts first, in dependency order.** The two To-verify components and the integration glue come before the full end-to-end run, so a failure is caught cheaply.

**Mandatory language decision (owner instruction, 2026-10-24):** all Temporal SDK code — workflows, workers, activities, adapters, CLI samples — **must be Golang** (chosen for speed and size). Temporal's multi-language support does not apply to this POC; every installation/deployment document carries this decision.

---

## M0 — Chassis
**Goal:** Proxmox 9.x + VMs (VM-Xa, VM-Xb, VM-2, VM-3) with Docker; RabbitMQ (Topic) and Temporal (Server + Worker) standing up and reachable.
**Done-when:**
- VMs provisioned; Docker pulls/runs a hello-container on each.
- RabbitMQ topic reachable from VM-Xa (publish) and VM-Xb (consume).
- Temporal Server up; a trivial workflow + worker runs to completion.
- Cross-VM reachability (VM-Xb → VM-2/VM-3, → GitHub) verified.
**Retires:** R8 (service discovery), R9 (VM numbering — lock it here), and the "does the chassis stand" risk.

## M1 — RabbitMQ → Temporal adapter
**Goal:** A RabbitMQ consumer that starts exactly one Temporal workflow per message (idempotent, no loss).
**Done-when:**
- Publish N messages → N distinct workflows start.
- Redeliver the same message (retry/duplicate) → no second workflow (idempotency key / correlation).
- No message lost on consumer restart (durable consumer + ack-after-start).
**Retires:** R2 (bridge correctness/loss).

## M2 — Container lifecycle manager
**Goal:** The custom core: a Temporal activity that `docker run`s an ephemeral executor, injects per-run config/credentials, and **reliably tears down** on success **and** failure/timeout.
**Done-when:**
- Spawn + hydrate + teardown of a dummy container via a Temporal activity.
- **Crash/timeout path verified:** a hung/failing container is still reaped; no leaked containers after M runs.
- Teardown is idempotent (re-running cleanup is safe).
- Image pre-pulled (not pulled at activity time) to bound boot latency.
**Retires:** R1 (lifecycle/leaks/crash recovery) — the biggest custom risk.

## M3 — Executor spikes (the two To-verify components)
**Goal:** Prove the two uncertain pieces inside the M2 lifecycle:
- **M3a Hermes ephemeral:** boot Hermes in a container; load persona + skills markdown from a git repo at startup; correct + fast.
- **M3b Claude Code headless:** run Claude Code in a container against a cloned repo; make + commit code with a **scoped** token.
**Done-when:**
- M3a: agent boots with the right persona/skills; boot time acceptable; run is stateless.
- M3b: coder produces a real code change and a git commit using a short-lived, branch-limited token.
**Retires:** R5 (Hermes persona-load), R3/R4 (concurrency + credential scoping at the coder).

## M4 — Memory + knowledge
**Goal:** Wire the three hydration sources: Hindsight R/W API, LLM-Wiki RAG, Available Agent List (git).
**Done-when:**
- An ephemeral agent **reads and writes** Hindsight per session over the network (R6).
- LLM-Wiki serves a relevant design-standard/skill retrieval on a query (R7).
- Agent loads its full persona + skills markdown from the Available Agent List repo.
**Retires:** R6, R7.

## M5 — End-to-end single flow + hardening
**Goal:** The full Version 1 flow: Engineer → Gateway → Manager → Topic → Temporal → ephemeral agents → hydrate → code to Project Repo → **destroy**.
**Done-when:**
- One real request end-to-end, including teardown, leaving **zero** orphan containers.
- Concurrency model chosen and validated (one coder per task vs. N parallel; repo-write strategy).
- Credential scoping in place (no long-lived owner token inside containers).
**Retires:** the "does it all compose" risk; locks the open concurrency + isolation items.

---

## Sequencing rationale
- **M0** is a dependency for everything (the chassis).
- **M1** proves the decoupling path before any executor work.
- **M2** is the largest custom piece and gates the executors — so it precedes M3.
- **M3** retires the two To-verify components early enough that, if one fails, it is still cheap to swap.
- **M4** adds knowledge/memory only after the executor runs.
- **M5** composes the whole flow and locks the remaining open design items.

**Gate:** proceed from M(n) to M(n+1) only when M(n)'s done-when criteria are met. A failed gate in M3 is the point of last cheap exit.

## Delivery plans (per milestone)

Each milestone has its own delivery plan in `delivery plans/`:

| Milestone | Delivery plan |
|---|---|
| M0 — Chassis | [delivery plans/M0-Chassis-Delivery-Plan.md](delivery%20plans/M0-Chassis-Delivery-Plan.md) |
| M1 — RabbitMQ → Temporal adapter | [delivery plans/M1-RabbitMQ-Temporal-Adapter-Delivery-Plan.md](delivery%20plans/M1-RabbitMQ-Temporal-Adapter-Delivery-Plan.md) |
| M2 — Container lifecycle manager | [delivery plans/M2-Container-Lifecycle-Manager-Delivery-Plan.md](delivery%20plans/M2-Container-Lifecycle-Manager-Delivery-Plan.md) |
| M3 — Executor spikes | [delivery plans/M3-Executor-Spikes-Delivery-Plan.md](delivery%20plans/M3-Executor-Spikes-Delivery-Plan.md) |
| M4 — Memory + knowledge | [delivery plans/M4-Memory-and-Knowledge-Delivery-Plan.md](delivery%20plans/M4-Memory-and-Knowledge-Delivery-Plan.md) |
| M5 — End-to-end + hardening | [delivery plans/M5-End-to-End-Hardening-Delivery-Plan.md](delivery%20plans/M5-End-to-End-Hardening-Delivery-Plan.md) |

Each delivery plan has a matching **full installation/deployment instruction set** in `installation-deployment/`:

| Milestone | Installation & deployment |
|---|---|
| M0 — Chassis | [installation-deployment/M0-Chassis-Installation-and-Deployment.md](installation-deployment/M0-Chassis-Installation-and-Deployment.md) |
| M1 — RabbitMQ → Temporal adapter | [installation-deployment/M1-Adapter-Installation-and-Deployment.md](installation-deployment/M1-Adapter-Installation-and-Deployment.md) |
| M2 — Container lifecycle manager | [installation-deployment/M2-Lifecycle-Installation-and-Deployment.md](installation-deployment/M2-Lifecycle-Installation-and-Deployment.md) |
| M3 — Executor spikes | [installation-deployment/M3-Executors-Installation-and-Deployment.md](installation-deployment/M3-Executors-Installation-and-Deployment.md) |
| M4 — Memory + knowledge | [installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md](installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md) |
| M5 — End-to-end + hardening | [installation-deployment/M5-End-to-End-Installation-and-Deployment.md](installation-deployment/M5-End-to-End-Installation-and-Deployment.md) |

## What this plan does **not** decide
It sequences de-risking; it does **not** finalize the open design items from the Version 1 foundation doc (VM numbering, exact RabbitMQ↔Temporal wiring, LLM-Wiki curation process, ephemeral isolation level, N+1 semantics, message shapes). Those get resolved *at* the relevant milestones (R9 at M0, R3/R4 at M3/M5) rather than up front.

---

*Companions: `Technical-Feasibility-Analysis.md` (the rating + verdict this plan de-risks), and the six per-milestone files in `delivery plans/`.*
