# Initial Foundation Design Document — Ephemeral AI Dev Agent System

| | |
|---|---|
| **Status** | Initial Foundation — POC baseline |
| **Date** | 2026-10-24 |
| **Source** | `AI Enterprise Team - Page 12.pdf`, diagram page titled "Single Flow POC - Ai Dev Agent Flow" |
| **Companion** | `ARCHITECTURE.md` (quick-reference map of the same diagram) |

**Provenance statement.** This document records, in detail, everything the source diagram contains: its zones, components, every connection with its label, the two note boxes verbatim, and the flow as read from the arrows. It deliberately contains **no** content beyond the diagram — no goals, principles, lifecycle models, payload schemas, or technology rationales of its own. It is the stable baseline from which design improvement iterations begin; anything added in later iterations should be new, clearly-marked content, not silently mixed into this record.

---

## 1. What the diagram conveys

The diagram depicts a **single-flow proof of concept** for an AI development agent pipeline:

- A **human Software Engineer** (Human-In-The-Loop, on Hermes Desktop) sits **outside** the On-Prem Company Boundary and initiates the flow.
- Inside the boundary, a **persistent manager** (Hermes Agent Gateway + Manager Agent on VM-Xa) receives the request and publishes it to a **message topic** (RabbitMQ, Topic) on VM-Xb.
- A **task queue and executor** (Temporal Server + Temporal Worker containers, on VM-Xb — a Proxmox 9.x server) consumes the topic; the worker's **"Executes Activity"** step spawns the working agents.
- The working agents are **ephemeral Docker containers** (Hermes Agent, Claude Code Agent (N+1)), drawn inside a red-dashed box to mark them as disposable.
- They hydrate from three knowledge sources: the **LLM-Wiki** (VM-2), the **Available Agent List** GitHub repo, and **Long-Term Memory (Hindsight)** (VM-3).
- The **Claude Code agent writes source code** to the **Project Repo** on cloud GitHub.
- Per the diagram's own Note 2, when the tasks are done the agents **and their containers are destroyed**.

The title carries the scope: **one flow**, proof of concept.

## 2. Zones

| Zone | Marking | Contents |
|---|---|---|
| **On-Prem Company Boundary** | large light-gray dashed box, label top-left | Everything on-prem: VM-Xa, VM-Xb, VM-2, VM-3 |
| **On-Prem / Cloud divider** | thin solid orange line | separates the boundary from the cloud zone; "On-Prem" labeled just below the boundary, "Cloud" below that |
| **Cloud** | bottom zone | the GitHub Repo box |
| **Outside the boundary** | far left | the Software Engineer (Human-In-The-Loop) with the "Hermes Desktop" label — the only actor drawn outside the company boundary |

## 3. Component inventory (exactly as drawn)

### VM-Xa (dashed box)
| Component | Drawing detail |
|---|---|
| **Hermes Agent (Docker Container)** | solid box inside VM-Xa |
| — **Hermes Agent Gateway** | vertical bar on the left side of the Hermes Agent box; arrow target of the Software Engineer |
| — **Manager Agent** | smaller box with a green robot icon inside the Hermes Agent box |

### VM-Xb — labeled "Proxmox 9.x Server" (dashed box)
| Component | Drawing detail |
|---|---|
| **Temporal Server Container** | top row, left; solid arrow from RabbitMQ (Topic) |
| **Temporal Worker Container** | top row, right; solid arrow to the Temporal Server labeled **"Polls Task Queue"** |
| **RabbitMQ (Topic)** | middle row; receives the green dashed arrow from the Manager Agent |
| **"Executes Activity"** | decision diamond between the worker and the ephemeral row; two solid arrows fan out to both ephemeral containers |
| **Hermes Agent (Ephemeral Docker Container)** | bottom row, left, human/laptop icon, inside the red-dashed box |
| **Claude Code Agent (N+1) (Ephemeral Docker Container)** | bottom row, right, red robot icon, inside the red-dashed box |

### VM-3 (dashed box)
| Component | Drawing detail |
|---|---|
| **Long-Term Memory (Hiindsight)** | source spelling; reads "Hindsight"; blue dashed **"R/W Memory"** arrows to the Hermes ephemeral container and down to the LLM-Wiki |

### VM-2 (dashed box)
| Component | Drawing detail |
|---|---|
| **LLM-Wiki** | blue dashed arrows up to both ephemeral containers and down to the GitHub repos |

### Cloud — GitHub Repo (solid black box; source label "Gitub Repo")
| Sub-box | Drawing detail |
|---|---|
| **Available Agent List** | solid box; blue dashed arrow up labeled **"Load Agent Skill"**; per Note 1 holds the agent index, the full agent persona, the tool list, and the markdown files with the Behavioral Specifications |
| **Project Repo** | solid box; blue dashed arrow down from Claude Code Agent (N+1) labeled **"Write Source Code to Repo"** |

## 4. Connection map (every arrow, as drawn)

| # | From | To | Label on the arrow | Line style |
|---|---|---|---|---|
| 1 | Software Engineer (Hermes Desktop) | Hermes Agent Gateway | — | solid black |
| 2 | Manager Agent | RabbitMQ (Topic) | — | green dashed |
| 3 | RabbitMQ (Topic) | Temporal Server Container | — | solid black |
| 4 | Temporal Worker Container | Temporal Server Container | "Polls Task Queue" | solid black |
| 5 | Temporal Worker Container | "Executes Activity" diamond | — | solid black |
| 6 | "Executes Activity" diamond | Hermes Agent (Ephemeral) | — | solid black |
| 7 | "Executes Activity" diamond | Claude Code Agent (N+1) | — | solid black |
| 8 | LLM-Wiki (VM-2) | Hermes Agent (Ephemeral) | "Read Agent Skills for Ephemeral Agent Spin-up" | blue dashed |
| 9 | LLM-Wiki (VM-2) | Claude Code Agent (N+1) | "Read Design Standards, Tools, and Skills From LLM-Wiki" | blue dashed |
| 10 | Available Agent List (GitHub) | Hermes Agent (Ephemeral) | "Load Agent Skill" | blue dashed |
| 11 | Long-Term Memory / Hindsight (VM-3) | Hermes Agent (Ephemeral) | "R/W Memory" | blue dashed |
| 12 | Long-Term Memory / Hindsight (VM-3) | Claude Code Agent (N+1) | "R/W Memory" | blue dashed |
| 13 | LLM-Wiki (VM-2) | Long-Term Memory (VM-3) | — | blue dashed |
| 14 | LLM-Wiki (VM-2) | GitHub repos | — | blue dashed |
| 15 | Claude Code Agent (N+1) | Project Repo (GitHub) | "Write Source Code to Repo" | blue dashed |

**Line/color semantics as drawn:** solid black lines carry the task/control flow; the green dashed line is the Manager Agent's publish into the topic; blue dashed lines are knowledge reads and memory R/W; the **red-dashed box** around the two ephemeral containers marks them as temporary units.

## 5. The diagram's two notes (verbatim; source typos kept)

> **Note 1:** The "Manager Agent" will look in the "Available Agent List" repo and search the agent index to find the full agent persona, list and tools that will be able to pull the markdown files and create the appropriate agent.f. So the agent will pass the repo location, and repo folder that contains all the markdowns, as well as the Behavioral Specifications.

> **Note 2:** On load-up, the ephemeral agents will pull the full agent profile markdowns from the repository, load those profiles, receive the behavioral specifications, then run the required tasks and then destroy the agent and container.

Reading of Note 1: the Manager Agent resolves agent definitions from the Available Agent List repo — the index lookup yields the full persona, tool list, and the markdown files; the Manager then passes the repo location and the repo folder containing the markdowns, plus the Behavioral Specifications, into the spin-up.

Reading of Note 2: at load-up the ephemeral agents pull their full profile markdowns, load the profiles, receive the behavioral specifications, run the required tasks, and **then the agent and its container are destroyed**.

Suspected source typos (kept verbatim above): "agent.f" (Note 1), "Hiindsight" (VM-3 box label), "Gitub Repo" (cloud box label).

## 6. The flow, read from the arrows

1. **Request in:** Software Engineer on Hermes Desktop → Hermes Agent Gateway (arrow 1).
2. **Publish:** Manager Agent → RabbitMQ (Topic) (arrow 2, green dashed).
3. **Enqueue:** RabbitMQ (Topic) → Temporal Server (arrow 3); the Temporal Worker polls the server's task queue (arrow 4, "Polls Task Queue").
4. **Spawn:** Temporal Worker → "Executes Activity" diamond → both ephemeral containers start (arrows 5–7).
5. **Hydrate:** LLM-Wiki → Hermes ephemeral ("Read Agent Skills for Ephemeral Agent Spin-up", arrow 8); LLM-Wiki → Claude Code ("Read Design Standards, Tools, and Skills From LLM-Wiki", arrow 9); Available Agent List → Hermes ephemeral ("Load Agent Skill", arrow 10); Hindsight ↔ both agents ("R/W Memory", arrows 11–12).
6. **Work:** the Claude Code Agent (N+1) performs the development task.
7. **Deliver:** Claude Code Agent (N+1) → Project Repo ("Write Source Code to Repo", arrow 15).
8. **Teardown:** per Note 2, the agents and their containers are destroyed.

## 7. What the diagram states, and what it leaves open

**Stated by the diagram itself:**
- the scope is one flow (title), human-initiated, human-reported
- the manager is persistent (VM-Xa); the executors are ephemeral (VM-Xb, red-dashed, destroyed after the task)
- the manager decouples from execution through a topic (arrow 2)
- the topic feeds a durable task queue (Temporal) whose worker polls and executes the activity
- agent identity/profiles live in the Available Agent List repo and are pulled at load-up, with behavioral specifications passed by the manager (Note 1, Note 2)
- knowledge is split across LLM-Wiki (shared standards/tools/skills) and Hindsight (long-term memory, R/W)
- the artifact of the flow is source code written to the Project Repo
- VM naming as drawn: VM-Xa, VM-Xb, VM-2 (LLM-Wiki), VM-3 (Hindsight)

**Left open by the diagram (candidates for design-iteration work; no position taken here):**
- the VM numbering scheme (this page assigns Hindsight=VM-3, LLM-Wiki=VM-2 — lock or renumber before naming infrastructure)
- the exact wiring between the RabbitMQ topic and the Temporal queue
- the queue/orchestration path's long-term shape for this project
- who curates the LLM-Wiki, and whether/what Hindsight learnings promote into it
- the isolation level of the ephemeral containers (co-located on VM-Xb vs. per-task VMs)
- the "N+1" scaling semantics (one coder per task, or N parallel coders)
- the message shapes between the topic and the queue

These open items are deliberately listed *only* — resolving them is the starting work for improving the design, and their resolutions belong in new, clearly-marked content rather than in this baseline record.
