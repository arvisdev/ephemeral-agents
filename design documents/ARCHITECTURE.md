# Single Flow POC — AI Dev Agent Flow

**Main architecture document** for the ephemeral-agents project.
Source: `AI Enterprise Team - Page 12.pdf` (architecture diagram). Typos in the source corrected here: "Hiindsight" → Hindsight, "Gitub" → GitHub, "agent.f" → agent.yaml.

## Overview

A single-flow proof of concept for an AI Dev Agent system. A human Software Engineer (Hermes Desktop, human-in-the-loop) initiates a dev task; a Hermes Manager Agent (VM-Xa) orchestrates it via a RabbitMQ topic into a Temporal server/worker pair (VM-Xb, Proxmox 9.x); the worker executes an activity that spins up two **ephemeral** Docker containers — a Hermes Agent and a Claude Code Agent (N+1); they load their personas/skills from a shared LLM-Wiki and an "Available Agent List" GitHub repo, do the work, write source to the Project Repo, R/W Long-Term Memory (Hindsight), then the agents and containers are destroyed.

## Zones and components

### On-Prem Company Boundary (dashed box)

**VM-Xa** (dashed box)
- **Hermes Agent (Docker Container)** containing:
  - **Hermes Agent Gateway** — entry point; receives the request from the Software Engineer (Human-In-The-Loop) on Hermes Desktop (the engineer sits *outside* the boundary, on the far left)
  - **Manager Agent** (robot icon)

**VM-Xb — Proxmox 9.x Server** (dashed box)
- **Task queue row (Docker containers):**
  - **Temporal Server Container**
  - **Temporal Worker Container** — solid arrow to the Temporal Server labeled **"Polls Task Queue"**
- **Messaging (Docker container):**
  - **RabbitMQ (Topic)** — solid arrow up into the Temporal Server Container; green dashed arrow from the Manager Agent (VM-Xa)
- **Ephemeral agents row** (red-dashed box = temporary):
  - **Hermes Agent (Ephemeral Docker Container)**
  - **Claude Code Agent (N+1) (Ephemeral Docker Container)**
  - The Temporal Worker points down into a **"Executes Activity"** decision diamond, which fans out with two solid arrows to both ephemeral containers

**Memory and knowledge stores**
- **VM-3** (dashed box): **Long-Term Memory (Hindsight)** — blue dashed "R/W Memory" arrows to the Hermes ephemeral agent and down to the LLM-Wiki
- **VM-2** (dashed box): **LLM-Wiki** — shared design standards, tools, and skills; blue dashed "R/W Memory" arrows to the ephemeral agents and down to the GitHub repos

### Cloud

- **GitHub Repo** (solid black box) containing:
  - **Available Agent List** — agent index: full agent persona/profile markdowns, tool lists, behavioral specifications
  - **Project Repo** — where source code is written

## Numbered flow

1. **Initiation** — Software Engineer on Hermes Desktop (human-in-the-loop, outside the boundary) sends a request to the Hermes Agent Gateway on VM-Xa.
2. **Orchestration** — the Manager Agent publishes a message to the RabbitMQ (Topic) container.
3. **Task queuing** — the Temporal Server picks up the message and enqueues it in its task queue.
4. **Worker polling** — the Temporal Worker polls the Temporal Server for tasks ("Polls Task Queue").
5. **Activity execution** — the Worker's "Executes Activity" step spins up the two ephemeral containers: Hermes Agent and Claude Code Agent (N+1).
6. **Agent setup and context loading** —
   - *Read Agent Skills for Ephemeral Agent Spin-up*: LLM-Wiki → Hermes Agent (Ephemeral)
   - *Load Agent Skill*: Available Agent List repo → Hermes Agent (Ephemeral) (pulls the full agent profile markdowns, tool lists, behavioral specifications; creates the appropriate agent.yaml)
   - *Read Design Standards, Tools, and Skills From LLM-Wiki*: LLM-Wiki → Claude Code Agent (N+1)
   - *R/W Memory*: Long-Term Memory (Hindsight) ↔ ephemeral agents
7. **Task execution** — the Claude Code Agent performs the development task, guided by the Hermes ephemeral agent.
8. **Result and memory** — *Write Source Code to Repo*: Claude Code Agent (N+1) → Project Repo (cloud GitHub); learned information goes to Hindsight (VM-3).
9. **Teardown** — the ephemeral agents and their containers are destroyed.

## Notes (verbatim from the source diagram)

> **Note 1:** The "Manager Agent" will look in the "Available Agent List" repo and search the agent index to find the full agent persona, list and tools that will be able to pull the markdown files and create the appropriate agent.yaml. So the agent will pass the repo location, and repo folder that contains all the markdowns, as well as the Behavioral Specifications.

> **Note 2:** On load-up, the ephemeral agents will pull the full agent profile markdowns from the repository, load those profiles, receive the behavioral specifications, then run the required tasks and then destroy the agent and container.

## Open questions (to reconcile before the POC is frozen)

1. **VM numbering** — this POC page assigns Hindsight = VM-3 and LLM-Wiki = VM-2; the confirmed main design (diagram -4) assigns Hindsight = VM-5 and LLM-Wiki = VM-3. Which numbering is authoritative?
2. **Task-queue path** — this POC uses RabbitMQ topics + Temporal server/worker as its queue and orchestration layer. Confirm whether this is the intended long-term path for this project, or a POC simplification that a heavier multi-flow queue layer will replace later.
