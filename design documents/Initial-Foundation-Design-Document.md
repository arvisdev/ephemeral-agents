# Initial Foundation Design Document — Ephemeral AI Dev Agent System

| | |
|---|---|
| **Status** | Initial Foundation — POC baseline |
| **Date** | 2026-10-24 |
| **Source** | `AI Enterprise Team - Page 12.pdf`, diagram page titled "Single Flow POC - AI Dev Agent Flow" |
| **Companion** | `ARCHITECTURE.md` (quick-reference map of the same diagram) |

**Provenance statement.** This document records, in detail, everything the source diagram contains: its zones and nesting, every component, every connection with its label, the two note boxes verbatim, and the flow as read from the arrows. It deliberately contains **no** content beyond the diagram — no goals, principles, lifecycle models, payload schemas, or technology rationales of its own. It is the stable baseline from which design improvement iterations begin; anything added in later iterations should be new, clearly-marked content, not silently mixed into this record.

**Verification.** All items below were re-checked against the source PDF rendered at 400 DPI. Items the rendering does not settle unambiguously are flagged **[confirm]** and collected in Section 8 rather than asserted.

---

## 1. What the diagram conveys

The diagram depicts a **single-flow proof of concept** for an AI development agent pipeline:

- A **human Software Engineer** (Human-In-The-Loop, on Hermes Desktop) sits **outside** the On-Prem Company Boundary and initiates the flow.
- Inside the boundary, the **Proxmox 9.x Server** hosts the virtual machines: **VM-Xa** carries the persistent manager side (Hermes Agent Gateway + Hermes Agent (Docker Container) with the Manager Agent inside), and **VM-Xb** carries the queue and execution side (Temporal Server Container, Temporal Worker Container, RabbitMQ (Topic), the "Executes Activity" diamond, and the ephemeral agents).
- Two more dashed boxes, **VM-3** (Long-Term Memory, Hindsight) and **VM-2** (LLM-Wiki), sit inside the company boundary but outside the Proxmox box.
- The "Executes Activity" step spawns two **ephemeral Docker containers** — Hermes Agent and Claude Code Agent (N+1) — drawn inside a red-dashed box to mark them as disposable.
- They hydrate from the **LLM-Wiki** (VM-2), the **Available Agent List** repo, and **Long-Term Memory (Hindsight)** (VM-3).
- The **Claude Code agent writes source code** to the **Project Repo** in the cloud "Github Repo".
- Per the diagram's own Note 2, when the tasks are done the agents **and their containers are destroyed**.

The title carries the scope: **one flow**, proof of concept.

## 2. Zones and nesting (exactly as drawn)

| Level | Box | Border | Contents |
|---|---|---|---|
| 1 (outermost) | **On-Prem Company Boundary** | large light-gray dashed box, label top-left | everything on-prem |
| 2 | **Proxmox 9.x Server** | dashed box nested inside the boundary | VM-Xa and VM-Xb |
| 3 | **VM-Xa** | dashed box inside Proxmox | Hermes Agent Gateway (vertical bar), Hermes Agent (Docker Container) with Manager Agent |
| 3 | **VM-Xb** | dashed box inside Proxmox | Temporal Server/Worker containers, RabbitMQ (Topic), "Executes Activity" diamond, red-dashed ephemeral row |
| 3 | **VM-3** | dashed box inside the boundary, outside Proxmox | Long-Term Memory (Hindsight) |
| 3 | **VM-2** | dashed box inside the boundary, outside Proxmox | LLM-Wiki |
| — | **External** | far left, outside the boundary | Software Engineer (Human-In-The-Loop) with the "Hermes Desktop" label — the only actor outside the company boundary |
| — | **On-Prem / Cloud divider** | thin solid orange line | "On-Prem" labeled above it, "Cloud" below |
| — | **Cloud** | bottom zone | the Github Repo box |

The red-dashed box inside VM-Xb encloses the two ephemeral agent containers — the diagram's own marking of the disposable units.

## 3. Component inventory (exactly as drawn)

### Proxmox 9.x Server — VM-Xa (dashed box)
| Component | Drawing detail |
|---|---|
| **Hermes Agent Gateway** | tall narrow vertical bar, text set vertically; arrow target of the Software Engineer |
| **Hermes Agent (Docker Container)** | solid box; the Manager Agent box sits inside it |
| **Manager Agent** | smaller box with a green robot icon; the note leader line ties to Note 1 |

### Proxmox 9.x Server — VM-Xb (dashed box)
| Component | Drawing detail |
|---|---|
| **Temporal Server Container** | "Docker Container" outer box, inner label "Temporal Server Container" |
| **Temporal Worker Container** | "Docker Container" outer box; arrow to the Temporal Server labeled **"Polls Task Queue"** |
| **RabbitMQ (Topic)** | "Docker Container" outer box (source label reads "RabbitMQ (Topic", closing paren appears missing); receives the green dashed arrow from the Manager Agent |
| **"Executes Activity"** | decision diamond between the worker and the ephemeral row; two solid arrows fan out to both ephemeral containers |
| **Hermes Agent (Ephemeral Docker Container)** | inside the red-dashed box, human/laptop icon |
| **Claude Code Agent (N+1) (Ephemeral Docker Container)** | inside the red-dashed box, red robot icon |

### VM-3 (dashed box, outside Proxmox, inside the boundary)
| Component | Drawing detail |
|---|---|
| **Long-Term Memory (Hindsight)** | blue dashed **"R/W Memory"** arrows to the ephemeral agents |

### VM-2 (dashed box, outside Proxmox, inside the boundary)
| Component | Drawing detail |
|---|---|
| **LLM-Wiki** | blue dashed arrows up to the ephemeral containers; a blue dashed line continues down toward the GitHub repos |

### Cloud — Github Repo (solid black box; source label "Github Repo", lowercase b)
| Sub-box | Drawing detail |
|---|---|
| **Available Agent List** | solid box; per Note 1 holds the agent index, the full agent persona, skill and tools, the markdown files, and the Behavioral Specification |
| **Project Repo** | solid box; blue dashed arrow down from Claude Code Agent (N+1) labeled **"Write Source Code to Repo"** |

## 4. Connection map (every arrow, as drawn)

**Task / control flow (solid & green dashed):**

| # | From | To | Label on the arrow | Line style |
|---|---|---|---|---|
| 1 | Software Engineer (Hermes Desktop) | Hermes Agent Gateway | — | solid |
| 2 | Manager Agent | RabbitMQ (Topic) | — | green dashed **[confirm: one reading shows it double-headed]** |
| 3 | RabbitMQ (Topic) | Temporal Server Container | — | solid |
| 4 | Temporal Worker Container | Temporal Server Container | "Polls Task Queue" | solid |
| 5 | Temporal Worker Container | "Executes Activity" diamond | — | solid |
| 6 | "Executes Activity" diamond | Hermes Agent (Ephemeral) | — | solid |
| 7 | "Executes Activity" diamond | Claude Code Agent (N+1) | — | solid |

**Knowledge / memory flows (blue dashed) — routing flagged where the rendering is ambiguous:**

| # | Label | Most plausible source → target | Confidence |
|---|---|---|---|
| 8 | "Read Agent Skills for Ephemeral Agent Spin-up" | LLM-Wiki (VM-2) → Hermes Agent (Ephemeral) | **[confirm]** |
| 9 | "Read Design Standards, Tools, and Skills From LLM-Wiki" | LLM-Wiki (VM-2) → the ephemeral agents | **[confirm]** |
| 10 | "Load Agent Skill" | Available Agent List (GitHub) → Hermes Agent (Ephemeral) | **[confirm: could originate at LLM-Wiki]** |
| 11 | "R/W Memory" | Long-Term Memory / Hindsight (VM-3) ↔ Hermes Agent (Ephemeral) | high |
| 12 | "R/W Memory" | Long-Term Memory / Hindsight (VM-3) ↔ Claude Code Agent (N+1) | high |
| 13 | (unlabeled) | LLM-Wiki (VM-2) → Long-Term Memory (VM-3) | medium |
| 14 | (unlabeled) | LLM-Wiki (VM-2) → GitHub repos | medium |
| 15 | "Write Source Code to Repo" | Claude Code Agent (N+1) → Project Repo (GitHub) | high |

**Line/color semantics as drawn:** solid black lines carry the task/control flow; the green dashed line is the Manager Agent's publish into the topic; blue dashed lines are knowledge reads and memory R/W; the **red-dashed box** around the two ephemeral containers marks them as temporary units; the orange divider separates On-Prem from Cloud.

## 5. The diagram's two notes (verbatim; source wording kept)

> **Note 1:** The "Manager Agent" will look in the "Available Agent List" repo and search the agent index to find the full agent persona, skill and tools that will be able to pull the markdown files and create the appropriate agent.f So the agent will pass the repo location, and repo folder that contains all the markdowns, as well as the Behavioral Specification.

> **Note 2:** On load up, the ephemeral agents will pull the full agent profile markdowns from the repository, load those profiles, receive the behavioral specifications, then run the required tasks and then destroy the agent and container.

**Reading of Note 1:** the Manager Agent resolves agent definitions from the Available Agent List repo — the index lookup yields the full persona, skill and tools, and the markdown files; the Manager then passes the repo location and the repo folder containing the markdowns, plus the Behavioral Specification, into the spin-up.

**Reading of Note 2:** at load-up the ephemeral agents pull their full profile markdowns, load the profiles, receive the behavioral specifications, run the required tasks, and **then the agent and its container are destroyed**.

**Source rendering notes (as drawn, not corrected):** "agent.f" in Note 1 is an anomalous token — likely intended as "agent.yaml" or "agent file". The cloud box label is "Github Repo" (lowercase b). "RabbitMQ (Topic" appears to be missing its closing parenthesis. "Hindsight" and "Github" are spelled as-is in the source.

## 6. The flow, read from the arrows

1. **Request in:** Software Engineer on Hermes Desktop → Hermes Agent Gateway (arrow 1).
2. **Publish:** Manager Agent → RabbitMQ (Topic) (arrow 2, green dashed).
3. **Enqueue:** RabbitMQ (Topic) → Temporal Server (arrow 3); the Temporal Worker polls the server's task queue (arrow 4, "Polls Task Queue").
4. **Spawn:** Temporal Worker → "Executes Activity" diamond → both ephemeral containers start (arrows 5–7).
5. **Hydrate:** LLM-Wiki → agents ("Read Agent Skills for Ephemeral Agent Spin-up", arrow 8; "Read Design Standards, Tools, and Skills From LLM-Wiki", arrow 9); Available Agent List → Hermes ephemeral ("Load Agent Skill", arrow 10); Hindsight ↔ both agents ("R/W Memory", arrows 11–12).
6. **Work:** the Claude Code Agent (N+1) performs the development task.
7. **Deliver:** Claude Code Agent (N+1) → Project Repo ("Write Source Code to Repo", arrow 15).
8. **Teardown:** per Note 2, the agents and their containers are destroyed.

## 7. What the diagram states, and what it leaves open

**Stated by the diagram itself:**
- the scope is one flow (title), human-initiated, human-reported
- the manager is persistent (VM-Xa, inside Proxmox 9.x Server); the executors are ephemeral (VM-Xb, red-dashed, destroyed after the task)
- the manager decouples from execution through a topic (arrow 2)
- the topic feeds the Temporal queue whose worker polls and executes the activity
- agent identity/profiles live in the Available Agent List repo and are pulled at load-up, with the Behavioral Specification passed by the manager (Note 1, Note 2)
- knowledge is split across LLM-Wiki (shared standards/tools/skills) and Hindsight (long-term memory, R/W)
- the artifact of the flow is source code written to the Project Repo
- VM naming as drawn: VM-Xa and VM-Xb (inside the Proxmox 9.x Server box), VM-2 (LLM-Wiki), VM-3 (Hindsight)

**Left open by the diagram (candidates for design-iteration work; no position taken here):**
- the VM numbering scheme as a whole (this page assigns Hindsight=VM-3, LLM-Wiki=VM-2 — lock or renumber before naming infrastructure)
- the exact wiring between the RabbitMQ topic and the Temporal queue
- the queue/orchestration path's long-term shape for this project
- who curates the LLM-Wiki, and whether/what Hindsight learnings promote into it
- the isolation level of the ephemeral containers (co-located on VM-Xb vs. per-task VMs)
- the "N+1" scaling semantics (one coder per task, or N parallel coders)
- the message shapes between the topic and the queue

These open items are deliberately listed *only* — resolving them is the starting work for improving the design, and their resolutions belong in new, clearly-marked content rather than in this baseline record.

## 8. Items to confirm (ambiguous in the source rendering)

Marked **[confirm]** in Sections 3–4:

1. **Arrow 2** — is the Manager Agent → RabbitMQ (Topic) arrow one-way, or double-headed (bidirectional)?
2. **Arrow 8** — "Read Agent Skills for Ephemeral Agent Spin-up": confirm source (LLM-Wiki?) and target (Hermes ephemeral?).
3. **Arrow 9** — "Read Design Standards, Tools, and Skills From LLM-Wiki": confirm the target (Hermes ephemeral, Claude Code, or both?).
4. **Arrow 10** — "Load Agent Skill": confirm whether it originates at the Available Agent List repo or at the LLM-Wiki.
