# M4 Delivery Plan — Agent Memory & Knowledge Access (Hindsight + llm-wiki + Agent Catalog)

| | |
|---|---|
| **Milestone** | M4 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | Wire the three hydration sources into the running lifecycle: **Hindsight** long-term memory (VM-3, R/W API), **llm-wiki** (the git repo `github.com/arvisdev/llm-wiki`, design standards + dev/business best practices — D-wiki-git, 2026-11-04: the LLM-Wiki VM/RAG service is removed), and the **Available Agent List** repo (agent skills/personas). |
| **Depends on** | M2 (lifecycle), M3a (an agent that actually hydrates) |
| **Retires risk** | R6 (Hindsight R/W API), R7 (llm-wiki curation — serving-layer risk retired by the git replacement; cadence/ownership still open) |

Store-content model (owner-confirmed): Hindsight = long-term memory; **llm-wiki git repo** = design standards and anything guiding development/business best practices; Git repo = anything the agents load.

## Work packages

**WP-4.1 — Hindsight service (VM-3)**
- Postgres + pgvector container on VM-3 with the memory store schema (sessions → entries, vector embeddings, metadata). Separate from Temporal's Postgres (M0 note).
- **API contract** (the key deliverable — it is what "R/W Memory" in the diagram means concretely):
  - `read(session_key, scope)` → memory entries relevant to the current task/agent
  - `write(session_key, entry, embedding)` → persist a learning/artifact at task end
  - Scoping rules: what one task's writes are visible to later tasks (cross-task) vs. private to the run.
- Deliverable: API contract doc + working service + a client helper the executor containers use over the network.

**WP-4.2 — llm-wiki consumption (git repo, D-wiki-git)**
- Reference standards live in the **llm-wiki** git repo — https://github.com/arvisdev/llm-wiki (private; scaffolded with `design-standards/`, `dev-best-practices/`, `business-best-practices/`, `tools-and-skills/`, `CHANGELOG.md`, README). **No wiki service, no retrieval API, no ingest pipeline** (the LLM-Wiki VM/RAG service was removed 2026-11-04).
- Hydration = a full **shallow clone** (`git clone --depth 1`) of llm-wiki at executor boot; agents read the markdown directly.
- Open item (recorded, not solved): the hydration/retrieval shape is undecided — the interim default is the full shallow clone above; a curated index/README-per-topic retrieval aid may be added later (V2). Curation cadence/ownership is tracked in the repo's CHANGELOG.

**WP-4.3 — Wire hydration into the lifecycle**
- M2's hydrate step now fans out to all three: llm-wiki shallow clone, Hindsight read, agent-catalog fetch (M3a's repo pull).
- At task end: Hindsight write (the "R/W" half) from the agent's report.
- Bounded hydration: record per-source latency; total hydration must stay within the M2 TTL headroom.

**WP-4.4 — Hydration test**
- One run through the M2 lifecycle showing: agent receives all three inputs (assert content present), writes a memory entry at task end, and a **second** task with related scope reads back what the first wrote (proves cross-task R/W).

## Deliverables

1. Hindsight service + **API contract doc** (read/write/scope semantics) + client helper.
2. llm-wiki consumption wiring: the shallow-clone hydration config (repo URL in lifecycle/executor config) + a curation-cadence note in the llm-wiki repo's CHANGELOG.
3. Updated lifecycle hydrate/teardown steps (M2 config gains the three source settings: llm-wiki repo URL, Hindsight endpoint, catalog repo + memory write-on-end).
4. Hydration test report (WP-4.4) including the cross-task read-back.

## Acceptance criteria (done-when)

1. **R/W over the network:** an ephemeral agent reads and writes Hindsight per session (WP-4.4, both directions).
2. **Cross-task:** task 2 sees task 1's memory write, per the scoping rules.
3. **llm-wiki clone + read:** an executor shallow-clones github.com/arvisdev/llm-wiki and finds the right standard/skill document for 3 test lookups (all correct; D-wiki-git — no retrieval API).
4. **Catalog load:** agent boot still loads its full persona/skills from the repo with the other sources live (regression).
5. **Bounded:** total hydration time recorded and within M2 TTL headroom.

## Exit gate

M5 may start when criteria 1–5 pass. If the Hindsight API contract needs a fundamentally different shape (e.g. no per-session scoping), record the adaptation in the decision log — the diagram's "R/W Memory" only requires session-scoped read + end-of-task write for POC scale.

## Notes

- VM-3 is provisioned in M0; this milestone puts its **real** service on it. There is **no wiki VM** (D-wiki-git): reference standards live in the llm-wiki git repo, consumed by clone.
- Keep content curation deliberately boring for the POC (markdown committed to the llm-wiki repo). R7's real question ("who curates, and what promotes from Hindsight into the llm-wiki?") is a Version 2 decision — the cadence/ownership note is tracked in the repo's CHANGELOG.
