# Single Flow POC — AI Dev Agent Flow

**Main architecture document** for the ephemeral-agents project.
Source: `AI Enterprise Team - ephemeral-agents.pdf` (single architecture diagram page).

**Provenance.** Every item below is transcribed from the diagram and was re-verified against the source PDF at high resolution. Where an item is a sequence reading of the arrows (not a labeled box), it is marked as such. The verbatim notes preserve the source's own wording. Items I could not resolve unambiguously from the rendering were flagged **[confirm]**; all four were resolved by the diagram owner on 2026-10-24 (see "Items confirmed" at the end of this document).

## Overview (read from the diagram)

A single-flow proof of concept for an AI Dev Agent system. A Software Engineer (Human-In-The-Loop, on Hermes Desktop, outside the company boundary) sends a request to the Hermes Agent Gateway on VM-Xa. The Manager Agent (VM-Xa) publishes to a RabbitMQ (Topic) on VM-Xb; the Temporal Server enqueues it and the Temporal Worker polls it ("Polls Task Queue"), then "Executes Activity" spins up two ephemeral Docker containers — Hermes Agent and Claude Code Agent (N+1; per the owner decision of 2026-10-25 the count is needs-based, not a fixed two — parallel containers for parallel requests under a hard team-config cap, POC rule of thumb ≤5). Both read context from the LLM-Wiki (VM-2), the Available Agent List repo, and Long-Term Memory / Hindsight (VM-3); the Claude Code agent writes source code to the Project Repo; per Note 2 the agents and containers are then destroyed.

## Zones and nesting (outermost → innermost)

```
On-Prem Company Boundary (outermost, dashed)
├── Proxmox 9.x Server (nested dashed box)
│   ├── VM-Xa (dashed)            → Hermes Agent Gateway + Hermes Agent (Docker Container) [Manager Agent]
│   └── VM-Xb (dashed)            → Temporal Server/Worker, RabbitMQ (Topic), "Executes Activity",
│                                     red-dashed ephemeral row [Hermes Agent, Claude Code Agent (N+1)]
├── VM-3 (dashed, outside Proxmox, inside boundary)  → Long-Term Memory (Hindsight)
└── VM-2 (dashed, outside Proxmox, inside boundary)  → LLM-Wiki

[external, far left]  Software Engineer (Human-In-The-Loop) on Hermes Desktop

Cloud (below the On-Prem / Cloud divider)
└── Github Repo (solid box)  → Available Agent List + Project Repo
```

Key structural point: **"Proxmox 9.x Server" is a parent box that contains both VM-Xa and VM-Xb.** VM-2 and VM-3 are separate dashed boxes that sit outside the Proxmox box but still inside the company boundary.

## Component inventory

### Proxmox 9.x Server
| Component | Note |
|---|---|
| **VM-Xa** (dashed box) | contains the gateway, the Hermes Agent container, and the Manager Agent |
| — **Hermes Agent Gateway** | tall narrow vertical bar, label set vertically; receives the engineer's request |
| — **Hermes Agent (Docker Container)** | solid box inside VM-Xa |
| — **Manager Agent** | sub-box inside the Hermes Agent container (green robot icon) |
| **VM-Xb** (dashed box) | hosts the Temporal pair, the topic, the activity diamond, and the ephemeral row |
| — **Temporal Server Container** | "Docker Container" box labeled "Temporal Server Container" |
| — **Temporal Worker Container** | "Docker Container" box; arrow to the Temporal Server labeled **"Polls Task Queue"** |
| — **RabbitMQ (Topic)** | "Docker Container" box; receives the green dashed arrow from the Manager Agent |
| — **"Executes Activity"** | decision diamond; two solid arrows fan out to the two ephemeral containers |
| — **Hermes Agent (Ephemeral Docker Container)** | inside the red-dashed ephemeral box |
| — **Claude Code Agent (N+1) (Ephemeral Docker Container)** | inside the red-dashed ephemeral box |

### VM-3 (dashed box, outside Proxmox)
- **Long-Term Memory (Hindsight)** — blue dashed **"R/W Memory"** arrows to the ephemeral agents

### VM-2 (dashed box, outside Proxmox)
- **LLM-Wiki** — blue dashed arrows to the ephemeral agents and toward the GitHub repos

### Cloud — Github Repo (solid black box; source label "Github Repo", lowercase b)
- **Available Agent List** — the agent index (per Note 1: full agent persona, skill and tools, the markdown files, and the Behavioral Specification)
- **Project Repo** — where source code is written

## Connection map (all arrows as drawn)

**Task / control flow (solid & green dashed):**

| From | To | Label | Style |
|---|---|---|---|
| Software Engineer (Hermes Desktop) | Hermes Agent Gateway | — | solid |
| Manager Agent | RabbitMQ (Topic) | — (two-way: response returned when needed) | green dashed, bidirectional ↔ |
| RabbitMQ (Topic) | Temporal Server Container | — | solid |
| Temporal Worker Container | Temporal Server Container | "Polls Task Queue" | solid |
| Temporal Worker Container | "Executes Activity" diamond | — | solid |
| "Executes Activity" diamond | Hermes Agent (Ephemeral) | — | solid |
| "Executes Activity" diamond | Claude Code Agent (N+1) | — | solid |

**Knowledge / memory flows (blue dashed) — sources confirmed by owner (2026-10-24):**

| Label | Source → Target | Basis |
|---|---|---|
| "Read Agent Skills for Ephemeral Agent Spin-up" | Available Agent List (Github Repo) → Hermes Agent (Ephemeral) | owner-confirmed: agent-load content lives in the Git repo |
| "Read Design Standards, Tools, and Skills From LLM-Wiki" | LLM-Wiki (VM-2) → the ephemeral agents | owner-confirmed: design standards & dev/business best practices live in the LLM-Wiki |
| "Load Agent Skill" | Available Agent List (Github Repo) → Hermes Agent (Ephemeral) | owner-confirmed: agent-load content lives in the Git repo |
| "R/W Memory" | Long-Term Memory / Hindsight (VM-3) ↔ the ephemeral agents | owner-confirmed: long-term memory = Hindsight |
| "Write Source Code to Repo" | Claude Code Agent (N+1) → Project Repo (Github) | as drawn |
| (unlabeled, blue dashed) | LLM-Wiki (VM-2) → Long-Term Memory / Hindsight (VM-3) | as drawn (unlabeled) |
| (unlabeled, blue dashed) | LLM-Wiki (VM-2) → Github Repo | as drawn (unlabeled) |

Line conventions: solid = task/control flow; green dashed = Manager Agent's publish; blue dashed = knowledge reads and memory R/W; red-dashed box around the ephemeral row = the disposable units.

## Flow (read from the arrows)

1. Software Engineer on Hermes Desktop → Hermes Agent Gateway (request).
2. Manager Agent → RabbitMQ (Topic) (publish; green dashed).
3. RabbitMQ (Topic) → Temporal Server (enqueues into the task queue).
4. Temporal Worker ↔ Temporal Server ("Polls Task Queue").
5. Temporal Worker → "Executes Activity" → both ephemeral containers start.
6. Context loading: Available Agent List (Git) → Hermes ephemeral ("Read Agent Skills for Ephemeral Agent Spin-up"; "Load Agent Skill"); LLM-Wiki → agents ("Read Design Standards, Tools, and Skills From LLM-Wiki"); Hindsight ↔ agents ("R/W Memory").
7. Claude Code Agent (N+1) → Project Repo ("Write Source Code to Repo").
8. Per Note 2: the ephemeral agents and their containers are destroyed.

## Notes (verbatim from the source diagram; source wording kept)

> **Note 1:** The "Manager Agent" will look in the "Available Agent List" repo and search the agent index to find the full agent persona, skill and tools that will be able to pull the markdown files and create the appropriate agent.f So the agent will pass the repo location, and repo folder that contains all the markdowns, as well as the Behavioral Specification.

> **Note 2:** On load up, the ephemeral agents will pull the full agent profile markdowns from the repository, load those profiles, receive the behavioral specifications, then run the required tasks and then destroy the agent and container.

Source rendering notes: "agent.f" (Note 1) is an anomalous token as drawn — likely intended to read "agent.yaml" or "agent file". The "Github" box label is lowercase-b in the source. "RabbitMQ (Topic" appears to be missing its closing parenthesis in the drawing.

## Items confirmed (owner clarification, 2026-10-24)

The four **[confirm]** items above were resolved by the diagram owner:

1. **Manager Agent ↔ RabbitMQ (Topic)** is two-way (double-headed) — the green dashed line returns a response to the manager when one is needed.
2. **"Read Agent Skills for Ephemeral Agent Spin-up"** originates at the **Available Agent List (Git repo)**, not the LLM-Wiki.
3. **"Read Design Standards, Tools, and Skills From LLM-Wiki"** originates at the **LLM-Wiki** and serves the ephemeral agents.
4. **"Load Agent Skill"** originates at the **Available Agent List (Git repo)**, not the LLM-Wiki.

**Underlying store-content model (owner-confirmed):**
- **Hindsight (VM-3)** = Long-Term Memory (R/W).
- **LLM-Wiki (VM-2)** = Design Standards and anything that guides development and business best practices.
- **Git repo (Available Agent List)** = anything the agents load (agent skills / personas).
