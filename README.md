# ephemeral-agents

**A standalone proof-of-concept of an ephemeral AI Dev Agent system** — a
human-initiated, human-reported flow in which a durable orchestrator (Temporal)
drives short-lived, git-provisioned executor containers that hydrate from
shared knowledge, do a unit of development work, and are destroyed with
nothing left behind.

This repo is the **Version 1 POC documentation set**: a faithful record of the
source architecture diagram, a desktop feasibility analysis, and a full
de-risking plan — milestones, delivery plans, installation/deployment
instruction sets, gate test procedures, and complete install scripts — built
so that a junior developer can execute the POC end-to-end from this repo alone.

> **Scope note.** This project is **standalone**. It deliberately shares no
> architecture with any production pipeline; every reference to one has been
> removed from this repo. What is here is the POC, by itself.

## The core idea

> A **durable orchestrator** (Temporal) drives **ephemeral, git-provisioned
> executor containers**, with knowledge split across a shared RAG wiki, a
> vector long-term memory, and a git-hosted agent catalog.

One flow, read from the source diagram (full connection map in
[version 1/ARCHITECTURE.md](version%201/ARCHITECTURE.md)):

1. A **Software Engineer** (human-in-the-loop, outside the boundary) sends a
   request to the **Hermes Agent Gateway + Manager Agent** on `vm-xa`.
2. The manager publishes to a **RabbitMQ topic** on `vm-xb` (two-way — a
   response returns when needed); **Temporal** enqueues and polls it.
3. "Executes Activity" spins up **ephemeral Docker containers on `vm-xb` as
   needed** — a Hermes agent for agent-type work, a Claude Code agent when
   coding occurs (the diagram's "(N+1)" label means the count is *variable*,
   not a fixed two), and multiple containers in parallel when requests are
   concurrent. The team sets a hard limit in configuration; the POC rule of
   thumb is **max 5**, tuned to the VM's configuration (not an absolute).
4. The agents hydrate: agent personas/skills from the **Available Agent List
   (GitHub repo)**, design standards & best practices from the **LLM-Wiki**
   (`vm-llm-wiki`), and long-term memory from **Hindsight**
   (`vm-hindsight`, Postgres + pgvector).
5. The coder agent **writes source code to the Project Repo** with a scoped,
   short-lived credential.
6. Per the diagram's Note 2: the agents and containers are **destroyed** —
   the outcome (artifact pointer or error) returns to the human.

## Owner-confirmed decisions (recorded in the doc set)

| Decision | Content |
|---|---|
| **Store-content model** | Hindsight = long-term memory (R/W). LLM-Wiki = design standards + dev/business best practices. Git repo (Available Agent List) = everything the agents load (skills / personas). |
| **Manager ↔ RabbitMQ** | Two-way (double-headed): responses return to the manager when needed. |
| **RabbitMQ stays (owner decision 2026-10-29)** | The broker is kept **deliberately**, not by default: it is the V2 ingestion boundary — other (non-Temporal) systems will publish tasks into the topic alongside the manager, which a direct SDK start cannot serve. The M1 adapter's idempotency/loss machinery (TC-M1-03..06) is the cost of owning that boundary, and M1 is the milestone that proves it is manageable. |
| **Language mandate** | **All Temporal SDK code is Golang** — workflows, workers, activities, adapters, CLI samples (owner instruction: speed + size). Every install doc carries this. |
| **TTL ownership (owner-settled 2026-10-29)** | Hybrid: `ttl_s` is optional in `message-v1.json`, written **only** by the manager (sized from M3 B1 boot-time p50); when absent, M2's lifecycle applies `POC_TTL_DEFAULT=900s`; the reaper is authoritative either way (R1 backstop intact). |
| **VM naming (D-8)** | `VM-Xa / VM-Xb / VM-2 / VM-3` are **discussion labels, not commitments**. Each project defines its own names; this POC uses `vm-xa`, `vm-xb`, `vm-llm-wiki`, `vm-hindsight` (legend in [`deploy/m0/hosts.env.example`](deploy/m0/hosts.env.example)). |

## Feasibility status

**Technically feasible — no fundamental blocker**
([Technical-Feasibility-Analysis.md](version%201/Technical-Feasibility-Analysis.md)).
The effort concentrates in integration glue (RabbitMQ→Temporal adapter, the
ephemeral-container lifecycle manager) and two *To-verify* components
(Hermes boot-time persona load; headless Claude Code container). The POC
sequence below is the plan to prove the verdict **with evidence, not analysis**.

## The POC sequence — six gated milestones

Prove the risky parts first, in dependency order. **Gate rule: a milestone's
test procedure must PASS before the next milestone's installation begins.**

| # | Milestone | Proves | Gate test procedure |
|---|---|---|---|
| M0 | Chassis | 4 VMs + Docker + RabbitMQ topic + Temporal reachable; Go SDK smoke | [TP-M0](version%201/test%20procedures/M0-Chassis-Test-Procedure.md) |
| M1 | RabbitMQ → Temporal adapter | Exactly one workflow per message; idempotent; no loss | [TP-M1](version%201/test%20procedures/M1-Adapter-Test-Procedure.md) |
| M2 | Container lifecycle manager | Spawn/timeout/reap state machine; crash-safe reaper; no-pull deploy gate | [TP-M2](version%201/test%20procedures/M2-Lifecycle-Test-Procedure.md) |
| M3 | Executor spikes | Hermes boots with persona/skills (point of last cheap exit); headless Claude Code makes a real commit | [TP-M3](version%201/test%20procedures/M3-Executors-Test-Procedure.md) |
| M4 | Memory + knowledge | Hindsight & LLM-Wiki APIs; hydration wired in; **cross-task read-back** | [TP-M4](version%201/test%20procedures/M4-Memory-Knowledge-Test-Procedure.md) |
| M5 | End-to-end + hardening | Human-to-human E2E run, four failure injections, credential hardening, docs closed | [TP-M5](version%201/test%20procedures/M5-End-to-End-Test-Procedure.md) |

The full sequence, done-when criteria, risk register (R1–R9), and every
cross-link is in [Technical-Feasibility-POC-Milestones.md](version%201/Technical-Feasibility-POC-Milestones.md).

## Repository layout

```
├── version 1/                        The documentation set
│   ├── AI Enterprise Team - ephemeral-agents.pdf   Source diagram (single source of truth for the v1 docs)
│   ├── ARCHITECTURE.md               Quick-reference diagram record: zones, components, all 15 connections, flow, verbatim notes
│   ├── Initial-Foundation-Design-Document.md       Detailed diagram baseline + confirmed items + the D-8 naming decision
│   ├── Architectural-Design-Document.md            Full buildable design: components, contracts, lifecycle state machine, failure modes, open items
│   ├── Business-Requirements-Document.md           BRD: business objectives BO-1..7, stakeholders, FR/NFR requirements, business rules, acceptance criteria
│   ├── Technical-Feasibility-Analysis.md           Component ratings (Proven/Feasible/Needs-build/To-verify), risk register, verdict
│   ├── Technical-Feasibility-POC-Milestones.md     The M0–M5 plan: goals, done-when, gate rule, and links to every companion doc
│   ├── delivery plans/               One delivery plan per milestone (work packages, deliverables, acceptance criteria)
│   ├── installation-deployment/      One full install/deploy instruction set per milestone — each section carries a "why" rationale
│   └── test procedures/              One gate test procedure per milestone (TP-M0 … TP-M5)
├── deploy/                           The executable layer
│   ├── README.md                     Operator entry point: step sequence, gate rule, first-time setup, failure routing
│   ├── m0/install.sh … m5/install.sh Six complete install scripts (idempotent; --check-only; gate tests; evidence to deploy/mN/evidence/)
│   └── m0/hosts.env.example          Host-map template with the D-8 hostname → label legend
└── LICENSE
```

Deliberately **not in git** (see `.gitignore`): `*.env` secret files and
`_quarantine/` (install scripts of unverified provenance, held for owner
review), and any personal credential file.

## How to run the POC

1. Read [`deploy/README.md`](deploy/README.md) — it is the front door: first-time
   setup, the six-step sequence, and "where do I look when a step fails."
2. For each milestone, in order: delivery plan → installation/deployment doc
   → run `deploy/mN/install.sh` → run the milestone's gate test procedure →
   **only then** start the next milestone.

**The POC is complete when** (M5 install doc §9): the E2E run passed with
**zero** surviving POC containers; all four failure injections ended clean
and were reported to the human; credential scans show no long-lived owner
secret reachable from any container; and the Version 2 open-items list is
written. At that point the feasibility verdict is confirmed by evidence.

## What this POC deliberately does not decide

Concurrency execution model, message-shape contract, ephemeral isolation
level, and LLM-Wiki curation ownership are resolved **at** their owning
milestones (M3–M5) and recorded in decision logs — they are not pre-decided
here. The concurrency *semantics* were set by owner input (2026-10-25):
needs-based container counts, parallel containers for parallel requests, a
team-config hard limit (POC rule of thumb ≤5, VM-config dependent); what
remains to lock at M3 is the execution model (branch vs. worktree) and the
cap value. The resolutions (or explicit deferrals) form the **Version 2
open-items list**, closing the loop from this POC to the next design
iteration.
