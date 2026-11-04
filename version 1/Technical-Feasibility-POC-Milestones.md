# POC Milestones & De-risking Plan — Ephemeral AI Dev Agent System (Version 1)

| | |
|---|---|
| **Status** | De-risking plan (companion to `Technical-Feasibility-Analysis.md`) |
| **Date** | 2026-10-24 |
| **Purpose** | Sequenced milestones that de-risk the Version 1 architecture in dependency order. Each milestone has a **goal**, **done-when (success criteria)**, and the **risk(s) it retires**. Not a build spec — it is the order to *prove feasibility*, not to build the full system. |

Principle: **prove the risky parts first, in dependency order.** The two To-verify components and the integration glue come before the full end-to-end run, so a failure is caught cheaply.

**Mandatory language decision (owner instruction, 2026-10-24):** all Temporal SDK code — workflows, workers, activities, adapters, CLI samples — **must be Golang** (chosen for speed and size). Temporal's multi-language support does not apply to this POC; every installation/deployment document carries this decision.

---

## M0 – Infrastructure Foundation
**Goal:** Proxmox 9.x + VMs (VM-Xa, VM-Xb, VM-2, VM-3) with Docker; RabbitMQ (Topic) and Temporal (Server + Worker) standing up and reachable. *(D-wiki-git, 2026-11-04: 3-VM topology — VM-2/vm-llm-wiki is dropped; the wiki is the git repo github.com/arvisdev/llm-wiki, shallow-cloned at executor boot.)*
**Done-when:**
- VMs provisioned; Docker pulls/runs a hello-container on each.
- RabbitMQ topic reachable from VM-Xa (publish) and VM-Xb (consume).
- Temporal Server up; a trivial workflow + worker runs to completion.
- Cross-VM reachability (VM-Xb → VM-3, → GitHub) verified. *(D-wiki-git, 2026-11-04: VM-2 is dropped; reachability covers vm-xa/vm-xb/vm-hindsight only.)*
**Retires:** R8 (service discovery), R9 (VM naming — recorded here; owner-resolved 2026-10-25: labels are POC discussion devices, not commitments), and the "does the chassis stand" risk.

## M1 – Message-to-Workflow Bridge
**Goal:** A RabbitMQ consumer that starts exactly one Temporal workflow per message (idempotent, no loss).
**Done-when:**
- Publish N messages → N distinct workflows start.
- Redeliver the same message (retry/duplicate) → no second workflow (idempotency key / correlation).
- No message lost on consumer restart (durable consumer + ack-after-start).
**Retires:** R2 (bridge correctness/loss).

## M2 – Container Spawn & Reap System
**Goal:** The custom core: a Temporal activity that `docker run`s an ephemeral executor, injects per-run config/credentials, and **reliably tears down** on success **and** failure/timeout.
**Done-when:**
- Spawn + hydrate + teardown of a dummy container via a Temporal activity.
- **Crash/timeout path verified:** a hung/failing container is still reaped; no leaked containers after M runs.
- Teardown is idempotent (re-running cleanup is safe).
- Image pre-pulled (not pulled at activity time) to bound boot latency.
**Retires:** R1 (lifecycle/leaks/crash recovery) — the biggest custom risk.

## M3 – Agent Executor Proofs (the two To-verify components)
**Goal:** Prove the two uncertain pieces inside the M2 lifecycle:
- **M3a Hermes ephemeral:** boot Hermes in a container; load persona + skills markdown from a git repo at startup; correct + fast.
- **M3b Claude Code headless:** run Claude Code in a container against a cloned repo; make + commit code with a **scoped** token.
**Done-when:**
- M3a: agent boots with the right persona/skills; boot time acceptable; run is stateless.
- M3b: coder produces a real code change and a git commit using a short-lived, branch-limited token.
**Retires:** R5 (Hermes persona-load), R3/R4 (concurrency + credential scoping at the coder).

## M4 – Agent Memory & Knowledge Access
**Goal:** Wire the three hydration sources: Hindsight R/W API, **llm-wiki git repo** (shallow clone, D-wiki-git — the LLM-Wiki RAG service is removed), Available Agent List (git).
**Done-when:**
- An ephemeral agent **reads and writes** Hindsight per session over the network (R6).
- An agent shallow-clones **llm-wiki** (github.com/arvisdev/llm-wiki) and reads the relevant design-standard/skill document from the clone (R7).
- Agent loads its full persona + skills markdown from the Available Agent List repo.
**Retires:** R6, R7.

## M5 – Full System Integration Test
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

**Gate:** proceed from M(n) to M(n+1) only when M(n)'s **test procedure** (`test procedures/`) passes all its mandatory cases — the test procedure is the formal gate for the next milestone's installation. A failed gate in M3 is the point of last cheap exit.

## Delivery plans (per milestone)

Each milestone has its own delivery plan in `delivery plans/`:

| Milestone | Delivery plan |
|---|---|
| M0 – Infrastructure Foundation | [delivery plans/M0-Chassis-Delivery-Plan.md](delivery%20plans/M0-Chassis-Delivery-Plan.md) |
| M1 – Message-to-Workflow Bridge | [delivery plans/M1-RabbitMQ-Temporal-Adapter-Delivery-Plan.md](delivery%20plans/M1-RabbitMQ-Temporal-Adapter-Delivery-Plan.md) |
| M2 – Container Spawn & Reap System | [delivery plans/M2-Container-Lifecycle-Manager-Delivery-Plan.md](delivery%20plans/M2-Container-Lifecycle-Manager-Delivery-Plan.md) |
| M3 – Agent Executor Proofs | [delivery plans/M3-Executor-Spikes-Delivery-Plan.md](delivery%20plans/M3-Executor-Spikes-Delivery-Plan.md) |
| M4 – Agent Memory & Knowledge Access | [delivery plans/M4-Memory-and-Knowledge-Delivery-Plan.md](delivery%20plans/M4-Memory-and-Knowledge-Delivery-Plan.md) |
| M5 – Full System Integration Test | [delivery plans/M5-End-to-End-Hardening-Delivery-Plan.md](delivery%20plans/M5-End-to-End-Hardening-Delivery-Plan.md) |

Each delivery plan has a matching **full installation/deployment instruction set** in `installation-deployment/`:

| Milestone | Installation & deployment |
|---|---|
| M0 – Infrastructure Foundation | [installation-deployment/M0-Chassis-Installation-and-Deployment.md](installation-deployment/M0-Chassis-Installation-and-Deployment.md) |
| M1 – Message-to-Workflow Bridge | [installation-deployment/M1-Adapter-Installation-and-Deployment.md](installation-deployment/M1-Adapter-Installation-and-Deployment.md) |
| M2 – Container Spawn & Reap System | [installation-deployment/M2-Lifecycle-Installation-and-Deployment.md](installation-deployment/M2-Lifecycle-Installation-and-Deployment.md) |
| M3 – Agent Executor Proofs | [installation-deployment/M3-Executors-Installation-and-Deployment.md](installation-deployment/M3-Executors-Installation-and-Deployment.md) |
| M4 – Agent Memory & Knowledge Access | [installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md](installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md) |
| M5 – Full System Integration Test | [installation-deployment/M5-End-to-End-Installation-and-Deployment.md](installation-deployment/M5-End-to-End-Installation-and-Deployment.md) |

Each milestone also has a formal **test procedure** in `test procedures/` — the gate that must PASS before the next milestone's installation begins:

| Milestone | Test procedure (the gate) |
|---|---|
| M0 – Infrastructure Foundation | [test procedures/M0-Chassis-Test-Procedure.md](test%20procedures/M0-Chassis-Test-Procedure.md) |
| M1 – Message-to-Workflow Bridge | [test procedures/M1-Adapter-Test-Procedure.md](test%20procedures/M1-Adapter-Test-Procedure.md) |
| M2 – Container Spawn & Reap System | [test procedures/M2-Lifecycle-Test-Procedure.md](test%20procedures/M2-Lifecycle-Test-Procedure.md) |
| M3 – Agent Executor Proofs | [test procedures/M3-Executors-Test-Procedure.md](test%20procedures/M3-Executors-Test-Procedure.md) |
| M4 – Agent Memory & Knowledge Access | [test procedures/M4-Memory-Knowledge-Test-Procedure.md](test%20procedures/M4-Memory-Knowledge-Test-Procedure.md) |
| M5 – Full System Integration Test | [test procedures/M5-End-to-End-Test-Procedure.md](test%20procedures/M5-End-to-End-Test-Procedure.md) |

## Install scripts (one per milestone)

Each installation/deployment instruction set has a matching **complete installation bash script** in `deploy/` (repo root), one per milestone. Each script implements its install doc end-to-end: asset check → build → deploy → gate tests → evidence. They are idempotent and safe to re-run.

**Operator entry point:** `deploy/README.md` — the front door for whoever runs the installs: the step-by-step sequence, the gate rule, first-time setup, and "where do I look when a step fails." Every `install.sh` also carries a short *FOR THE OPERATOR* block in its header.

| Milestone | Install script |
|---|---|
| M0 – Infrastructure Foundation | [../deploy/m0/install.sh](../deploy/m0/install.sh) |
| M1 – Message-to-Workflow Bridge | [../deploy/m1/install.sh](../deploy/m1/install.sh) |
| M2 – Container Spawn & Reap System | [../deploy/m2/install.sh](../deploy/m2/install.sh) |
| M3 – Agent Executor Proofs | [../deploy/m3/install.sh](../deploy/m3/install.sh) |
| M4 – Agent Memory & Knowledge Access | [../deploy/m4/install.sh](../deploy/m4/install.sh) |
| M5 – Full System Integration Test | [../deploy/m5/install.sh](../deploy/m5/install.sh) |

**Common interface (all six):**

- `bash deploy/mN/install.sh` — full run: install + gate tests (evidence to `deploy/mN/evidence/`)
- `--check-only` — preconditions + asset check only, no changes
- `--no-gate` — run the install, but don't fail the script on gate-test failures
- `ASSUME_YES=1` — non-interactive (CI / re-runs)
- Hosts come from `deploy/m0/hosts.env` (auto-loaded; start from the committed `deploy/m0/hosts.env.example` — `hosts.env` itself is gitignored); secrets are generated into gitignored `.env` files, never committed
- A script exits 0 only when its gate checks pass (unless `--no-gate`) — the gate result is what the matching test procedure signs off on
- The install docs are the source of truth; the scripts implement them. Keep them in sync — a script/proc drift is a docs bug.

**Note:** scripts fail fast on missing assets and list exactly which files the delivery plan must produce (compose files, Dockerfiles, Go services, test harnesses) before the install can run.

## What this plan does **not** decide
It sequences de-risking; it does **not** finalize the open design items from the Version 1 foundation doc (exact RabbitMQ↔Temporal wiring, LLM-Wiki curation process, ephemeral isolation level, N+1 *execution* semantics (R3), message shapes). Those get resolved *at* the relevant milestones (R3/R4 at M3/M5) rather than up front. The N+1 *concurrency semantics* were settled by owner input on 2026-10-25 (needs-based container counts; parallel containers for parallel requests; team-config hard limit, POC rule of thumb ≤5, VM-config dependent — recorded at M3 §3); what remains open is the execution model + cap value. **VM naming (R9) is also settled** (owner 2026-10-25): the labels are POC discussion devices, not commitments — per-project naming is a deploy-time choice.

---

*Companions: `Technical-Feasibility-Analysis.md` (the rating + verdict this plan de-risks), the six per-milestone files in `delivery plans/`, the six installation/deployment instruction sets in `installation-deployment/`, the six gate test procedures in `test procedures/`, and the six complete installation bash scripts in `deploy/m0/`–`deploy/m5/` (`install.sh`).*
