# Single Flow POC — AI Dev Agent Flow

**Main architecture document** for the ephemeral-agents project.
Source: `AI Enterprise Team - Page 12.pdf` (single architecture diagram page).

**Provenance:** every item in this document is transcribed from the diagram. Where a step is described in sequence form, it is a reading of the diagram's arrows, not separate source material. Suspected typos in the source are kept in the verbatim notes and flagged.

## Overview (read from the diagram)

A single-flow proof of concept for an AI Dev Agent system. A Software Engineer (Human-In-The-Loop, on Hermes Desktop, outside the company boundary) sends a request to the Hermes Agent Gateway on VM-Xa. The Manager Agent (VM-Xa) publishes to a RabbitMQ (Topic) on VM-Xb; the Temporal Server enqueues it and the Temporal Worker polls it ("Polls Task Queue"), then "Executes Activity" spins up two ephemeral Docker containers — Hermes Agent and Claude Code Agent (N+1). Both load context from the LLM-Wiki (VM-2), the Available Agent List GitHub repo, and Long-Term Memory / Hindsight (VM-3); the Claude Code agent writes source code to the Project Repo; per Note 2, the agents and containers are then destroyed.

## Zones

| Zone | Contents |
|---|---|
| **External (far left, outside boundary)** | Software Engineer (Human-In-The-Loop) on Hermes Desktop |
| **On-Prem Company Boundary** (dashed box) | VM-Xa, VM-Xb (Proxmox 9.x server), VM-2, VM-3 |
| **Cloud** (below the On-Prem / Cloud divider) | GitHub Repo: *Available Agent List*, *Project Repo* |

## Component inventory

### VM-Xa (dashed box)
- **Hermes Agent (Docker Container)** containing:
  - **Hermes Agent Gateway** — receives the request from the Software Engineer on Hermes Desktop
  - **Manager Agent**

### VM-Xb — Proxmox 9.x Server (dashed box)
- **Task queue row (Docker containers):**
  - **Temporal Server Container**
  - **Temporal Worker Container** — arrow to the Temporal Server labeled **"Polls Task Queue"**
- **Messaging (Docker container):**
  - **RabbitMQ (Topic)** — solid arrow up into the Temporal Server Container; green dashed arrow from the Manager Agent (VM-Xa)
- **Ephemeral agents row** (red-dashed box):
  - **Hermes Agent (Ephemeral Docker Container)**
  - **Claude Code Agent (N+1) (Ephemeral Docker Container)**
  - The Temporal Worker points down into an **"Executes Activity"** decision diamond, which fans out with two solid arrows to both ephemeral containers

### Memory and knowledge stores
- **VM-3** (dashed box): **Long-Term Memory (Hindsight)** — blue dashed "R/W Memory" arrows to the Hermes ephemeral agent and down to the LLM-Wiki
- **VM-2** (dashed box): **LLM-Wiki** — blue dashed "R/W Memory" arrows to the ephemeral agents and down to the GitHub repos

### Cloud
- **GitHub Repo** (solid black box; labeled "Gitub Repo" in the source) containing:
  - **Available Agent List** — the agent index (per Note 1: full agent persona, list and tools, the markdown files, and the Behavioral Specifications)
  - **Project Repo** — where source code is written

## Connection map (all arrows as drawn)

| From | To | Label | Style |
|---|---|---|---|
| Software Engineer (Hermes Desktop) | Hermes Agent Gateway | — | solid black |
| Manager Agent | RabbitMQ (Topic) | — | green dashed |
| RabbitMQ (Topic) | Temporal Server Container | — | solid black |
| Temporal Worker Container | Temporal Server Container | "Polls Task Queue" | solid black |
| Temporal Worker Container | "Executes Activity" diamond | — | solid black |
| "Executes Activity" diamond | Hermes Agent (Ephemeral) | — | solid black |
| "Executes Activity" diamond | Claude Code Agent (N+1) | — | solid black |
| LLM-Wiki (VM-2) | Hermes Agent (Ephemeral) | "Read Agent Skills for Ephemeral Agent Spin-up" | blue dashed |
| LLM-Wiki (VM-2) | Claude Code Agent (N+1) | "Read Design Standards, Tools, and Skills From LLM-Wiki" | blue dashed |
| Available Agent List (GitHub) | Hermes Agent (Ephemeral) | "Load Agent Skill" | blue dashed |
| Long-Term Memory / Hindsight (VM-3) | Hermes Agent (Ephemeral) | "R/W Memory" | blue dashed |
| Long-Term Memory / Hindsight (VM-3) | Claude Code Agent (N+1) | "R/W Memory" | blue dashed |
| LLM-Wiki (VM-2) | Long-Term Memory (VM-3) | — | blue dashed |
| LLM-Wiki (VM-2) | GitHub repos | — | blue dashed |
| Claude Code Agent (N+1) | Project Repo (GitHub) | "Write Source Code to Repo" | blue dashed |

Line conventions: solid black = task/control flow; green dashed = Manager Agent's publish; blue dashed = knowledge reads and memory R/W; red-dashed box around the ephemeral row = the disposable units.

## Flow (read from the arrows)

1. Software Engineer on Hermes Desktop → Hermes Agent Gateway (request).
2. Manager Agent → RabbitMQ (Topic) (publish; green dashed).
3. RabbitMQ (Topic) → Temporal Server (enqueues into the task queue).
4. Temporal Worker ↔ Temporal Server ("Polls Task Queue").
5. Temporal Worker → "Executes Activity" → both ephemeral containers start.
6. Context loading: LLM-Wiki → both agents ("Read Agent Skills for Ephemeral Agent Spin-up"; "Read Design Standards, Tools, and Skills From LLM-Wiki"); Available Agent List → Hermes ephemeral ("Load Agent Skill"); Hindsight ↔ both agents ("R/W Memory").
7. Claude Code Agent (N+1) → Project Repo ("Write Source Code to Repo").
8. Per Note 2: the ephemeral agents and their containers are destroyed.

## Notes (verbatim from the source diagram; source typos kept)

> **Note 1:** The "Manager Agent" will look in the "Available Agent List" repo and search the agent index to find the full agent persona, list and tools that will be able to pull the markdown files and create the appropriate agent.f. So the agent will pass the repo location, and repo folder that contains all the markdowns, as well as the Behavioral Specifications.

> **Note 2:** On load-up, the ephemeral agents will pull the full agent profile markdowns from the repository, load those profiles, receive the behavioral specifications, then run the required tasks and then destroy the agent and container.

Suspected source typos: "agent.f" (Note 1), "Hiindsight" (VM-3 box label), "Gitub Repo" (cloud box label).
