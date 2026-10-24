# Initial Foundation Design Document — Ephemeral AI Dev Agent System

| | |
|---|---|
| **Status** | Initial Foundation — POC baseline |
| **Date** | 2026-10-24 |
| **Source** | "Single Flow POC – Ai Dev Agent Flow" diagram (`AI Enterprise Team - Page 12.pdf`) |
| **Companion** | `ARCHITECTURE.md` (transcription of the diagram); this document is the *interpretation* of it |
| **Purpose** | Capture the initial design thinking in full detail so it serves as the stable starting point for iterative design improvement |

**How to use this document.** Sections 2–5 state *why* the system exists and what it must be. Sections 6–13 are the *what*: components, identity model, lifecycle, sequence, knowledge plane, topology, and technology choices. Sections 14–16 track *what is unresolved*: security posture, open decisions, and the improvement backlog. Items marked **[D-n]** are open design decisions; items marked **[A-n]** are assumptions that hold for the POC and may be revisited. Each carries the context needed to make the decision.

---

## 1. Problem statement

A human software engineer (Software Engineer, HITL) initiates a development task from Hermes Desktop. The task must be completed by a team of AI agents — a coordinating Hermes agent plus one or more Claude Code worker agents — that:

1. Are **ephemeral**: spun up fresh for the task, loaded with their identity and context, do the work, and are destroyed.
2. Have **versioned, reusable identity**: their persona, behavioral specifications, tool lists, and skills live as markdown in a git repository ("Available Agent List"), not baked into containers or hardcoded.
3. Are **deterministically orchestrated**: a persistent manager agent on durable-execution infrastructure (Temporal) dispatches work, retries failures, and enforces timeouts.
4. **Share knowledge**: design standards and skills come from a shared LLM-Wiki; experience across tasks accumulates in a long-term memory (Hindsight).
5. **Deliver artifacts to source control**: completed source code is written to the project's GitHub repo.
6. Stay **inside the company boundary**: all agent, orchestration, and memory components run on-prem; only git source control and LLM access reach the cloud.

The POC demonstrates **one flow** end-to-end: one request → one coordinated agent pair (Hermes ephemeral + Claude Code N+1) → code in the repo → teardown. Multi-flow and channel-based generalization come later (Section 16).

## 2. Goals (POC)

- G1: End-to-end single flow, human-initiated through to code commit and container destruction.
- G2: Validate durable orchestration via RabbitMQ topic + Temporal server/worker as the POC's task-queue and execution path.
- G3: Validate the **pull-based agent identity model** (profiles, tools, behavioral specs pulled from the Available Agent List repo at spin-up; `agent.yaml` generated at load-up).
- G4: Validate shared knowledge (LLM-Wiki) and experiential memory (Hindsight) read/write patterns for ephemeral agents.
- G5: Confirm container lifecycle safety: every task's containers are destroyed on completion *and* on failure.

## 3. Non-goals (explicitly out of POC scope)

- Multi-flow concurrency and channel-based fan-out.
- High availability, multi-tenant isolation, load management.
- Cost optimization, agent pooling/warm-start, checkpoint/restore of agent state.
- Human approval gates mid-task (the POC has HITL only at initiation).
- UI beyond Hermes Desktop.

## 4. Design principles

1. **Human-in-the-loop is primary.** The engineer initiates and receives results; agents never self-direct work. The POC scope is one request at a time.
2. **Separate the manager from the executors.** The manager agent (VM-Xa) is *persistent* and *never does the work itself*; ephemeral containers (VM-Xb) are the workers. This keeps blast radius small and the manager cheap to run.
3. **Identity is data, not code.** An agent's persona, skills, tools, and behavioral specs are versioned markdown in git. Spinning up an agent = pulling its identity folder + generating its runtime manifest. This makes agent types reusable across projects and auditable (identity diffs show in git history).
4. **Executors are disposable.** No state lives in an agent container except the task result. Durable state belongs to Temporal (workflow state), the repos (artifacts + identity), LLM-Wiki (knowledge), and Hindsight (experience).
5. **Orchestration is durable.** Temporal provides retry, timeout, and replay semantics for the "Executes Activity" step — container failures re-run rather than silently drop.
6. **Topics decouple producer from consumers.** The manager publishes to a RabbitMQ topic; any consumer (Temporal in the POC, a heavier multi-flow queue layer later) can pick it up. The manager never knows who executes.
7. **Knowledge plane vs. experience plane.** LLM-Wiki holds *stable* knowledge (design standards, tool docs, skills); Hindsight holds *learned* knowledge (what worked, what failed, per agent type). Agents read both at spin-up; only sanctioned writes go back.
8. **Data stays on-prem; source control is in the cloud.** All components inside the On-Prem Company Boundary; the only external system is GitHub (identity + project repos).

## 5. System context

```
 OUTSIDE boundary                                   CLOUD
 ┌───────────────────┐                              ┌──────────────────────────┐
 │ Software Engineer │   Hermes Desktop             │  GitHub                  │
 │ (Human-In-The-    │──────────►  ┌──────────────On-Prem Company Boundary─────│  ├─ Available Agent List │
 │  Loop)            │             │  VM-Xa: Hermes Agent Docker Container     │  │   (agent index,        │
 └───────────────────┘             │   ├─ Hermes Agent Gateway                 │  │    personas, tools,    │
                                   │   └─ Manager Agent ──────────┐           │  │    behavioral specs)   │
                                   │                                ▼         │  └─ Project Repo           │
                                   │          RabbitMQ (Topic) ◄──┐  │        │       (source code)       │
                                   │  VM-Xb: Proxmox 9.x server   │  └──────┐ │                              │
                                   │   ├─ Temporal Server/Worker ◄┘         │ │  egress: git + LLM only     │
                                   │   └─ ephemeral containers ─────────────┼──┘─────────────────────────────┘
                                   │        Hermes Agent (ephemeral)                ▲
                                   │        Claude Code Agent (N+1)                │
                                   │ VM-2: LLM-Wiki ───── R/W ────┤                │
                                   │ VM-3: Hindsight (LT memory) ┘                │
                                   └────────────────────────────────────────────────┘
```

Actors: **Software Engineer** (human, outside the boundary), **Manager Agent** (orchestrator), **Ephemeral Hermes Agent** (task coordinator for this flow), **Claude Code Agent N+1** (code producer), **GitHub** (identity + artifact system of record).

---

## 6. Component inventory and responsibilities

| Component | Home (VM) | Type | Responsibilities | Explicitly NOT responsible for |
|---|---|---|---|---|
| **Hermes Agent Gateway** | VM-Xa | Service inside the Hermes Agent Docker Container | Entry point for engineer requests; admission; routing to Manager Agent | Queuing, workflow logic |
| **Manager Agent** | VM-Xa | Persistent Hermes agent | Resolve agent types from the Available Agent List; build the task spec (profile refs, behavioral spec, project repo, timeouts); publish to the RabbitMQ topic; own the Temporal workflow definition; track workflow state; report result back to the engineer | Executing agent containers, doing development work |
| **RabbitMQ (Topic)** | VM-Xb | Docker container | Message bus between manager and queue; topic-based pub/sub (fan-out ready) | Workflow state, retry policy |
| **Temporal Server** | VM-Xb | Docker container | Durable workflow engine; task queue; history/replay; retry + timeout enforcement | Spinning up containers |
| **Temporal Worker** | VM-Xb | Docker container | Polls the Temporal task queue ("Polls Task Queue"); executes the **Executes Activity** step: create/destroy the ephemeral containers; supervise the run; write results | Knowing *which* agent types to use (comes in the task spec) |
| **Hermes Agent (Ephemeral)** | VM-Xb | Ephemeral Docker container (red-dashed) | Task-level coordination for this flow: load its profile + behavioral spec, read agent skills for spin-up, coordinate the Claude Code worker, R/W Hindsight | Long-lived state; direct repo writes (the code producer does that) |
| **Claude Code Agent (N+1)** | VM-Xb | Ephemeral Docker container (red-dashed) | Read design standards/tools/skills from LLM-Wiki; perform the development task; **write source code to the Project Repo** | Orchestration of its own lifecycle |
| **LLM-Wiki** | VM-2 | Service | Shared knowledge base: design standards, tool documentation, skills (RAG); read source for spin-up and execution | Storing task experience (that is Hindsight) |
| **Long-Term Memory (Hindsight)** | VM-3 | Service (Vectify `hindsight.vectorize.io` + PostgreSQL/pgvector) | Experience store; **per-agent-type banks**; vector-similarity recall at spin-up; consolidation writes at teardown | Storing stable knowledge (that is LLM-Wiki) |
| **Available Agent List** (GitHub) | Cloud | Git repo | Agent index (catalog) + per-agent-type folders: persona/profile markdowns, tool lists, behavioral specifications | Task data, project code |
| **Project Repo** (GitHub) | Cloud | Git repo | System of record for the development artifact (source code) | Agent identity |

> **VM-Xa / VM-Xb are templates**, not singletons: VM-Xa carries the manager-agent role and VM-Xb the ephemeral-agent role; the POC instantiates one of each [A-1].

## 7. Agent identity model (pull-based profiles)

The core idea: **an agent type is a git folder; an agent instance is a fresh container that hydrates itself from that folder at boot.**

Structure of the Available Agent List repo [A-2]:

```
available-agent-list/
├── index.md (or index.json)          # agent catalog: type id → folder, summary, capability tags
└── agents/
    ├── <agent-type-a>/
    │   ├── SOUL.md                   # persona / character
    │   ├── PERSONA.md                # operating style
    │   ├── USER.md                   # user context conventions
    │   ├── SKILLS.md                 # capability + skill references
    │   ├── TOOLS.md                  # tool list and usage notes
    │   └── behavioral-spec.md        # hard constraints for task execution
    └── <agent-type-b>/…
```

Flow (diagram Note 1, cleaned): the Manager Agent queries the agent index, selects the full persona + tool list that can pull the markdown files, and **passes the repo location, the repo folder containing all the markdowns, and the behavioral specifications** to the spin-up. At load-up (Note 2) each ephemeral agent:

1. clones/pulls its profile folder from the Available Agent List repo,
2. loads the profiles and receives the behavioral specifications,
3. generates its **`agent.yaml`** runtime manifest (container config, model, tool grants, memory bank pointer, task refs),
4. runs the assigned task,
5. is destroyed with its container.

Reusability constraint inherited from the agent-standard work: profile markdowns are **project-independent** — no company names, no app-layer technology names; project-specific technology is expressed as placeholders ("the project's …"), so one agent type serves many projects. [D-1: pin this NFR formally; versioning strategy for profile folders — semver the type, or git tags per agent type.]

## 8. Agent lifecycle (state machine)

```
 Requested ──► Queued ──► Enqueued ──► Provisioning ──► Loading ──► Executing
    (manager)  (RabbitMQ) (Temporal)   (worker creates     (pull     (Claude Code
                                       containers)          profiles,    does the work)
                                                              LLM-Wiki,
                                                               Hindsight)
 Executing ──► Publishing ──► Consolidating ──► Destroyed
               (git push to     (Hindsight        (container teardown;
                Project Repo)     write-back)       guaranteed on all exits)
```

| State | Owner | Exit criteria | Failure semantics |
|---|---|---|---|
| Requested | Manager Agent | Valid task spec built | Reject at gateway; engineer sees error immediately |
| Queued | RabbitMQ topic | Message consumed | Message ack/retry per queue config [D-4: ack model — manual vs. auto] |
| Enqueued | Temporal Server | Activity scheduled | Durable — survives worker crash (replayed) |
| Provisioning | Temporal Worker | Containers up and healthy | Temporal retry with backoff; timeout → teardown |
| Loading | Ephemeral agents | Profiles + specs + context loaded into agent config | Retry the activity; a bad profile folder is a *data* error → fail fast, no retry storm [D-5: profile validation at publish time?] |
| Executing | Claude Code N+1 | Task work complete | Worker heartbeat to Temporal; mid-run crash → re-run activity (idempotent writes required → branch + PR, not main) [A-3] |
| Publishing | Claude Code N+1 | Code pushed to Project Repo | Push retry; artifact is idempotent (branch name from task id) |
| Consolidating | Hermes ephemeral | Hindsight write-back done | Best-effort, retried [A-4] |
| Destroyed | Temporal Worker | `docker rm` confirmed for all containers | Destroy on *every* exit path — success, failure, timeout; the red-dashed box in the diagram exists so no one forgets this |

## 9. End-to-end sequence (detailed)

| # | Actor → target | Channel | Action | Payload sketch |
|---|---|---|---|---|
| 1 | Engineer → Gateway | Hermes Desktop UI | Submit task request | task description, project repo, target agent type(s), constraints |
| 2 | Gateway → Manager Agent | in-container routing | Admit + forward | same, enriched with engineer id, timestamp |
| 3 | Manager Agent → Available Agent List | git (read) | Look up agent index; select profile folders + behavioral specs | agent type ids |
| 4 | Manager Agent → Temporal | workflow start | Register durable workflow | `workflowId = task-{uuid}` |
| 5 | Manager Agent → RabbitMQ (Topic) | topic publish | Dispatch task message | `{taskId, workflowId, agentSpecs: [{repo, folder, behavioralSpec}], projectRepo, timeout, retryPolicy}` |
| 6 | Temporal Server | queue | Enqueue activity from the topic feed | activity `StartEphemeralTask(taskMsg)` |
| 7 | Temporal Worker | poll loop | "Polls Task Queue" → claim activity | — |
| 8 | Temporal Worker → VM-Xb runtime | container orchestration | "Executes Activity": create Hermes-ephemeral + Claude-Code-N+1 containers | `agent.yaml` per agent |
| 9 | Hermes ephemeral → LLM-Wiki (VM-2) | RAG read | "Read Agent Skills for Ephemeral Agent Spin-up" | skill queries |
| 10 | Hermes ephemeral → Hindsight (VM-3) | vector recall | Read prior experience for this agent type + project | recall(top-k) |
| 11 | Claude Code N+1 → LLM-Wiki (VM-2) | RAG read | "Read Design Standards, Tools, and Skills" | standards/skill queries |
| 12 | Hermes ephemeral → Claude Code N+1 | local IPC | Task handoff with behavioral spec + recalled experience | task brief |
| 13 | Claude Code N+1 → Project Repo (GitHub) | git push | "Write Source Code to Repo" | branch `task-{taskId}` [A-3] |
| 14 | Hermes ephemeral → Hindsight (VM-3) | write | Consolidate: outcome, decisions, pitfalls → agent-type bank | episode record |
| 15 | Temporal Worker | activity complete | Report result to workflow | `{status, branch, diffRef}` |
| 16 | Temporal Worker | teardown | Destroy both containers (guaranteed) | — |
| 17 | Manager Agent → Engineer | Hermes Desktop | Result report (branch, summary, memory notes) | — |

[D-6] Steps 5→6: in the POC, RabbitMQ feeds the Temporal queue directly. Confirm the exact wiring (consumer that calls `workflow.start` vs. a shared-queue pattern) — this is the POC's key wiring decision to finalize before generalizing beyond the single flow.

## 10. Knowledge plane and memory plane

**LLM-Wiki (VM-2)** — *stable knowledge*, shared read-mostly:
- Content: design standards, tool documentation, reusable skills/how-tos, and shared agent skill material [A-5].
- Read: every spin-up (both agent types) and during execution (design standards for the coder).
- Write: owned by the human/manager side, not by workers [D-7: who curates LLM-Wiki, and how do POC learnings promote from Hindsight into it?].

**Hindsight (VM-3)** — *learned knowledge*, per-agent-type banks:
- Backend: Vectify (`hindsight.vectorize.io`) over PostgreSQL + pgvector.
- Read: at spin-up, vector recall scoped to the agent's type bank — "what worked/failed for me before."
- Write: at consolidation, episode records (task, decisions, outcome, pitfalls) into the agent-type bank.
- Bank scoping matters: a coder-type agent's memory should not pollute a coordinator-type agent's recall. [A-6: POC may use a single bank; type-scoping is a Phase-1 item.]

The two planes are deliberately distinct: standards never get *learned* (they are curated); experience never becomes *standard* without curation. The LLM-Wiki ⇄ Hindsight arrow in the diagram represents that curation path.

## 11. Deployment topology

- Hypervisor: **Proxmox 9.x**. VMs: **VM-Xa** (gateway + manager container), **VM-Xb** (Temporal server + worker + RabbitMQ + ephemeral containers), **VM-2** (LLM-Wiki), **VM-3** (Hindsight).
- Networking [A-7]: VM-Xa and VM-Xb share a L2 segment (topic feed is low-latency, no public exposure); VM-2/VM-3 reachable on internal service ports; the *only* egress from the boundary is GitHub (443) and the LLM provider.
- Container host: ephemeral agent containers run on VM-Xb's Docker daemon inside the worker's reach. N+1 semantics: one Claude Code container per task by default; the "(N+1)" label means the pattern scales to N additional coder containers for parallel subtasks [A-8].
- [D-8: the POC page numbers the memory VMs as Hindsight=VM-3, LLM-Wiki=VM-2. Lock in this project's VM numbering scheme before the POC hardcodes it in infrastructure.]
- [D-9: co-location of ephemeral containers on VM-Xb (cheap, POC-simple) vs. one VM per task (hard isolation). POC chooses co-location; the template-based VM-Xb design supports the latter later.]

## 12. Technology choices and rationale

| Choice | Role | Why | Risk / alternative |
|---|---|---|---|
| **Temporal** (server + worker containers) | Durable orchestration + activity execution | Retry, timeout, replay, and visibility out of the box; container-failure re-run without re-dispatch | The POC validates durable-orchestration semantics (durable, poll-based, activity-executed) that any future multi-flow pipeline should preserve |
| **RabbitMQ (Topic)** | Manager → queue handoff, fan-out ready | Simple pub/sub; one topic per flow for the POC | A later multi-channel scheme (one topic per flow) must be able to adopt the same message shape without a rewrite [D-10] |
| **Docker containers** | Agent isolation | Disposable units, image = agent runtime, no kernel sprawl on POC | Full VMs per task later if hard isolation is required (D-9) |
| **Hindsight / Vectify + PG/pgvector** | Per-agent-type experiential memory | Existing confirmed choice; vector recall with bank scoping | — |
| **Proxmox 9.x** | On-prem VM layer | Established estate | — |
| **Git repos (GitHub)** | Identity + artifacts | Identity-as-data principle; auditable, versioned, cloud source control is an accepted external | None in POC scope |

## 13. Interface contracts (sketches — to be hardened in Phase 1)

**Task topic message** (RabbitMQ; keep the schema transport-neutral):

```json
{
  "taskId": "task-01j…",
  "workflowId": "task-01j…",
  "agentSpecs": [
    {"type": "coordinator", "repo": "github.com/org/available-agent-list",
     "folder": "agents/coordinator-a", "agentYaml": "…/agents/coordinator-a/agent.yaml",
     "behavioralSpec": "…/agents/coordinator-a/behavioral-spec.md"},
    {"type": "coder", "repo": "…", "folder": "agents/coder-b", …}
  ],
  "projectRepo": "github.com/org/project-repo",
  "timeout": "2h",
  "retryPolicy": {"maxAttempts": 3, "backoff": "exponential"}
}
```

**Temporal activity** (executed by the worker):

```
StartEphemeralTask(taskMsg) → { status, containerRefs[], resultBranch }
   sub-steps: createContainers → loadContext → execute → publish → consolidate → destroy
```

**`agent.yaml`** (generated at load-up from the profile folder): identity (type, repo ref), model + provider, tool grants, memory bank pointer, task refs, behavioral-spec digest. [D-11: schema owner — manager-side codegen or per-type checked-in manifests.]

## 14. Security and operational posture

- **Secrets hygiene:** the POC's own lesson from day one — a PAT file (`github-pac.txt`) ended up in the project dir and had to be git-ignored. Generalize: agent containers receive credentials via the runtime (env injection from a secrets store), never via committed files; `.gitignore` ships with every repo we seed.
- **Least privilege per container:** coder containers need git (branch-scoped, no force-push) + LLM API; coordinator containers need RAG/memory access; neither gets the manager's keys.
- **Teardown guarantee:** destruction of containers is part of the activity's terminal step on *all* paths (success, error, timeout) — a leaked long-lived agent container is the exact failure mode the red-dashed box forbids.
- **Egress allow-list:** boundary egress limited to GitHub 443 + LLM provider; nothing else.
- **Auditability:** Temporal workflow history is the audit log; identity changes are git history; memory writes are episode records with provenance (taskId).
- **HITL boundary:** initiation and result acceptance are human; autonomous mid-task decisions live inside the behavioral spec and are its owner's accountability [A-9: POC behavioral spec = static markdown; per-task overrides are a later feature].

## 15. Open decisions and assumptions register

| ID | Topic | Options / context | Status |
|---|---|---|---|
| D-1 | Profile versioning strategy | Semver per agent type vs. git tags vs. branch-per-release | Open |
| D-4 | RabbitMQ ack model | Manual ack after Temporal workflow start vs. auto | Open |
| D-5 | Profile validation | Validate folder/schema at publish time (fail fast) vs. at load-up | Open |
| D-6 | RabbitMQ→Temporal wiring | Consumer-starts-workflow vs. shared queue | Open |
| D-7 | LLM-Wiki curation ownership + Hindsight→LLM-Wiki promotion path | Human-curation vs. semi-automated | Open |
| D-8 | VM numbering | Lock the scheme: keep the POC page's numbering (Hindsight=VM-3, LLM-Wiki=VM-2) or renumber | **Blocking — decide** |
| D-9 | Ephemeral isolation level | Co-located containers (POC) vs. per-task VMs (later) | POC = co-located |
| D-10 | Message-shape neutrality | Keep the topic schema transport-neutral so a multi-channel queue layer can adopt it later | Open |
| D-11 | `agent.yaml` provenance | Codegen at load-up vs. checked-in per-type manifest | Open |
| A-1 | One instance each of VM-Xa/VM-Xb in POC | Assumption | POC-held |
| A-2 | Available Agent List folder layout (`agents/<type>/{SOUL,PERSONA,USER,SKILLS,TOOLS,behavioral-spec}.md`) | Assumption, consistent with the agent-standard work | POC-held |
| A-3 | Code lands on branch `task-{taskId}`, not main | Assumption (retry-safe) | POC-held |
| A-4 | Hindsight write-back is best-effort | Assumption | POC-held |
| A-6 | Single memory bank in POC; per-type scoping in Phase 1 | Assumption | POC-held |
| A-7 | L2 segment VM-Xa↔VM-Xb; egress allow-list | Assumption | POC-held |
| A-8 | "N+1" = 1 coder by default, N for parallel subtasks | Assumption | POC-held |

## 16. Improvement backlog (starting point for design iteration)

**Phase 0 — this POC.** Implement the single flow as Section 9; harden the teardown guarantee (G5); instrument the activity with visibility; resolve D-8 (VM numbering) before any code or infra names VMs.

**Phase 1 — multi-flow generalization.** Replace the single RabbitMQ topic with a channel model (one topic per flow) and a durable pipeline feed in place of the POC's direct Temporal feed; per-type Hindsight banks; LLM-Wiki curation process (D-7); profile validation at publish (D-5).

**Phase 2 — multi-flow pipeline.** Adopt the transport-neutral message schema (D-10); generalize orchestration from single-flow to the multi-channel pipeline; decide per-task VM isolation (D-9); define N+1 scaling policy and resource ceilings.

**Phase 3 — scale and hardening.** Agent pooling/warm-start, mid-task HITL gates, cost observability, memory curation automation, drift detection between LLM-Wiki standards and what agents actually produce.

---

## Appendix A — component → VM map

| Component | VM |
|---|---|
| Hermes Agent (gateway + manager) | VM-Xa |
| RabbitMQ topic, Temporal server/worker, ephemeral containers | VM-Xb |
| LLM-Wiki | VM-2 ⚠ |
| Hindsight | VM-3 ⚠ |

⚠ = numbering for this project not yet locked (D-8, blocking).

## Appendix B — glossary

- **HITL** — human-in-the-loop. **POC** — proof of concept. **Bank** — an agent-type-scoped slice of Hindsight. **Episode record** — one write-back: task, decisions, outcome, pitfalls. **Behavioral spec** — the hard-constraints markdown an agent must obey at task time. **agent.yaml** — the runtime manifest generated at load-up from the profile folder.

## Appendix C — sources and transcription notes

- Source diagram: `AI Enterprise Team - Page 12.pdf`, page "Single Flow POC – Ai Dev Agent Flow". Transcription: `ARCHITECTURE.md`.
- OCR caveats from the source: "Hiindsight"→Hindsight, "Gitub"→GitHub, "agent.f"→`agent.yaml`, "Polls Task Queue" misread as "Polis" in one pass, "the appropriate agent" contextually "agent.yaml".
- Both verbatim notes (Manager Agent profile lookup; ephemeral load-up → run → destroy) are preserved in `ARCHITECTURE.md`.
