# ephemeral-agents

**Author:** Ronald A Steinhauer — A&R Visual Development Inc (arvisdev) ·
**Authorship & provenance:** see [AUTHORSHIP.md](AUTHORSHIP.md) — the
concept, designs, and specifications are the author's original work
(conceived May 13, 2026); AI served as directed research assistant, document
creator, and rules-bound implementer.

## Where to start

**Read the [book](book/README.md) first.** It is the primary resource for
understanding this project: Part I (Chapters 1–3) explains the problem, the
thesis, and the organization in plain language — no technical background
needed; Part II (Chapters 4–10) walks through every component and why it was
designed the way it was; Part III (Chapters 11–13) is the proof — how it was
built, what failed, and what was learned; Chapter 14 tells an operator how to
deploy the whole system from scratch.

After the book:

1. [Initial-POC-Findings-Report.md](Initial-POC-Findings-Report.md) — the
   signed evidence record of the proof-of-concept (M0–M5).
2. [POC-Retrospective-and-Insights.md](POC-Retrospective-and-Insights.md) —
   the practitioner's lessons digest.
3. [`version 1/`](version%201/) — the V1 design and specification set
   (the original documents, kept as baselined records).
4. [`deploy/`](deploy/README.md) — the executable layer: install scripts and
   gate tests, for actually running the system.

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
> executor containers**, with knowledge split across a git-hosted reference
> wiki (shallow-cloned from a git repo), a vector long-term memory, and a
> git-hosted agent catalog.

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
4. The agents hydrate: agent personas/skills from the **agent roster
   (`agent-roster` — AGENT ROSTER (ephemeral-org), the machine-readable
   catalog since 2026-11-08; superseding the planned Available Agent List)**,
   design standards & best practices from the **llm-wiki
   git repo** (per D-wiki-git, 2026-11-04: no wiki VM/service — the wiki is
   the private git repo `github.com/arvisdev/llm-wiki`, shallow-cloned at
   executor boot; no retrieval API), and long-term memory from **Hindsight**
   (`vm-hindsight`, Postgres + pgvector).
5. The coder agent **writes source code to the Project Repo** with a scoped,
   short-lived credential.
6. Per the diagram's Note 2: the agents and containers are **destroyed** —
   the outcome (artifact pointer or error) returns to the human.

## Owner-confirmed decisions (recorded in the doc set)

| Decision | Content |
|---|---|
| **Store-content model** | Hindsight = long-term memory (R/W). llm-wiki git repo = design standards + dev/business best practices (D-wiki-git, 2026-11-04: RAG service removed; plain git repo, shallow-cloned). Git repo (`agent-roster`) = everything the agents load (skills / personas). |
| **Manager ↔ RabbitMQ** | Two-way (double-headed): responses return to the manager when needed. |
| **RabbitMQ stays (owner decision 2026-10-29)** | The broker is kept **deliberately**, not by default: it is the V2 ingestion boundary — other (non-Temporal) systems will publish tasks into the topic alongside the manager, which a direct SDK start cannot serve. The M1 adapter's idempotency/loss machinery (TC-M1-03..06) is the cost of owning that boundary, and M1 is the milestone that proves it is manageable. |
| **Language mandate** | **All Temporal SDK code is Golang** — workflows, workers, activities, adapters, CLI samples (owner instruction: speed + size). Every install doc carries this. |
| **TTL ownership (owner-settled 2026-10-29)** | Hybrid: `ttl_s` is optional in `message-v1.json`, written **only** by the manager (sized from M3 B1 boot-time p50); when absent, M2's lifecycle applies `POC_TTL_DEFAULT=900s`; the reaper is authoritative either way (R1 backstop intact). |
| **VM naming (D-8)** | `VM-Xa / VM-Xb / VM-2 / VM-3` are **discussion labels, not commitments**. Each project defines its own names; this POC uses `vm-xa`, `vm-xb`, `vm-hindsight` (legend in [`deploy/m0/hosts.env.example`](deploy/m0/hosts.env.example); D-wiki-git, 2026-11-04: `vm-llm-wiki` dropped — 3-VM topology). |

## Feasibility status

**PROVEN — all six milestones (M0–M5) completed and signed 2026-11-07.**
The gate results and evidence are recorded in
[Initial-POC-Findings-Report.md](Initial-POC-Findings-Report.md); the
practitioner's lessons are in
[POC-Retrospective-and-Insights.md](POC-Retrospective-and-Insights.md); and
the full thesis → design → proof treatment is the
[book](book/README.md) (14 chapters).

Original feasibility verdict (pre-build):
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
| M0 | Infrastructure Foundation | 3 VMs + Docker + RabbitMQ topic + Temporal reachable; Go SDK smoke | [TP-M0](version%201/test%20procedures/M0-Chassis-Test-Procedure.md) |
| M1 | Message-to-Workflow Bridge | Exactly one workflow per message; idempotent; no loss | [TP-M1](version%201/test%20procedures/M1-Adapter-Test-Procedure.md) |
| M2 | Container Spawn & Reap System | Spawn/timeout/reap state machine; crash-safe reaper; no-pull deploy gate | [TP-M2](version%201/test%20procedures/M2-Lifecycle-Test-Procedure.md) |
| M3 | Agent Executor Proofs | Hermes boots with persona/skills (point of last cheap exit); headless Claude Code makes a real commit | [TP-M3](version%201/test%20procedures/M3-Executors-Test-Procedure.md) |
| M4 | Agent Memory & Knowledge Access | Hindsight API + llm-wiki git clone (D-wiki-git); hydration wired in; **cross-task read-back** | [TP-M4](version%201/test%20procedures/M4-Memory-Knowledge-Test-Procedure.md) |
| M5 | Full System Integration Test | Human-to-human E2E run, four failure injections, credential hardening, docs closed | [TP-M5](version%201/test%20procedures/M5-End-to-End-Test-Procedure.md) |

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
│   ├── Available-Agent-List-Repository.md          Contract for the external agent-catalog repo: decision (D-catalog), layout, index.json shape, change rules
│   ├── Store-Usage-Rules.md                        What goes where: Hindsight (memory) vs llm-wiki git repo (reference, D-wiki-git) vs git (exact artifacts) — the go/not-go rules per store
│   ├── Technical-Feasibility-Analysis.md           Component ratings (Proven/Feasible/Needs-build/To-verify), risk register, verdict
│   ├── Technical-Feasibility-POC-Milestones.md     The M0–M5 plan: goals, done-when, gate rule, and links to every companion doc
│   ├── delivery plans/               One delivery plan per milestone (work packages, deliverables, acceptance criteria)
│   ├── installation-deployment/      One full install/deploy instruction set per milestone — each section carries a "why" rationale
│   ├── NFR-1-Gap-Closure-Plan.md                   Gap inventory + 4-phase plan to satisfy the verbatim NFR-1 requirement (junior dev completes full install+test unaided)
│   └── test procedures/              One gate test procedure per milestone (TP-M0 … TP-M5)
├── schemas/                          Versioned inter-zone contracts
│   └── message-v1.json               The task message envelope (authoritative per D-msg-v1; owned by the M1 delivery plan)
├── book/                             The book: Part I thesis (ch1–3), Part II design (ch4–10), Part III proof (ch11–13), ch14 build-from-scratch; README = table of contents
├── Initial-POC-Findings-Report.md    The signed evidence record of the POC (M0–M5)
├── POC-Retrospective-and-Insights.md The practitioner's lessons digest (effort, failure taxonomy, V2 priorities)
├── deploy/                           The executable layer
│   ├── README.md                     Operator entry point: step sequence, gate rule, first-time setup, failure routing
│   ├── m0/install.sh … m5/install.sh Six complete install scripts (idempotent; --check-only; gate tests; evidence to deploy/mN/evidence/)
│   └── m0/hosts.env.example          Host-map template with the D-8 hostname → label legend
└── LICENSE
```

Deliberately **not in git** (see `.gitignore`): `*.env` secret files, gate
evidence (`deploy/m*/evidence/`, created at run), build artifacts, and any
personal credential file.

## Versioning conventions (V1 → V2 readiness)

> **V2 kickoff (2026-11-08).** Version 2 has started in its own
> **private** repository: [`ephemeral-agents-v2`](https://github.com/arvisdev/ephemeral-agents-v2).
> This repository is the **frozen public V1 record** — no new content is
> added here. V2 cites this record by commit hash
> (`14bf62dc81729ed0ece355c30b5b6888d4fbf058`); see the V2 repo's
> `decision-log.md` for the kickoff decision.

The repo is structured so a Version 2 iteration can start without a
reorganization debate:

- **`version 1/` is the V1 design record, and it is closed to new content.**
  The baseline rule: nothing is added to, or merged into, a version's doc set
  after its content is baselined — each iteration gets its own folder. V2
  design docs go in `version 2/` when that iteration starts, never into
  `version 1/`.
- **`deploy/`, `services/`, and `schemas/` serve the *current* iteration**
  (there is only ever one live executable layer). At V2 kickoff, `deploy/`
  freezes as the V1 evidence trail — its gate runs and evidence outputs are
  the proof record — and V2 gets its own parallel deploy/services trees.
  That placement decision is recorded in the V2 open-items list at kickoff,
  not in advance.
- **`schemas/` holds the live inter-zone contracts**, versioned by filename
  (`message-v1.json` → `message-v2.json`). A V2 contract is added alongside;
  the V1 contract file is retired only when no V1 consumer remains, and every
  schema copy (see the M1 drift guard) is compared to its authoritative file.
- **Decision rows are per-version.** V1 decisions live in the V1 docs'
  decision logs; a V2 decision never re-litigates a resolved V1 row — it
  cites it and supersedes it in V2's own log.

## How to run the POC

1. Read [`deploy/README.md`](deploy/README.md) — it is the front door: first-time
   setup, the six-step sequence, and "where do I look when a step fails."
2. For each milestone, in order: delivery plan → installation/deployment doc
   → run `deploy/mN/install.sh` → run the milestone's gate test procedure →
   **only then** start the next milestone.

**The POC is complete when** (M5 install doc section 9): the E2E run passed with
**zero** surviving POC containers; all five failure injections ended clean
and were reported to the human; credential scans show no long-lived owner
secret reachable from any container; and the Version 2 open-items list is
written ([`version 1/Version-2-Open-Items.md`](version%201/Version-2-Open-Items.md)).
At that point the feasibility verdict is confirmed by evidence.

## What this POC deliberately does not decide

Concurrency execution model, message-shape contract, ephemeral isolation
level, and llm-wiki curation ownership (D-wiki-git: curation = markdown
commits to the wiki repo) are resolved **at** their owning
milestones (M3–M5) and recorded in decision logs — they are not pre-decided
here. The concurrency *semantics* were set by owner input (2026-10-25):
needs-based container counts, parallel containers for parallel requests, a
team-config hard limit (POC rule of thumb ≤5, VM-config dependent); what
remains to lock at M3 is the execution model (branch vs. worktree) and the
cap value. The resolutions (or explicit deferrals) form the **Version 2
open-items list**, closing the loop from this POC to the next design
iteration.
