# M4 Delivery Plan — Memory + Knowledge (Hindsight + LLM-Wiki + Agent Catalog)

| | |
|---|---|
| **Milestone** | M4 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | Wire the three hydration sources into the running lifecycle: **Hindsight** long-term memory (VM-3, R/W API), **LLM-Wiki** (VM-2, design standards + dev/business best practices), and the **Available Agent List** repo (agent skills/personas). |
| **Depends on** | M2 (lifecycle), M3a (an agent that actually hydrates) |
| **Retires risk** | R6 (Hindsight R/W API), R7 (LLM-Wiki curation) |

Store-content model (owner-confirmed): Hindsight = long-term memory; LLM-Wiki = design standards and anything guiding development/business best practices; Git repo = anything the agents load.

## Work packages

**WP-4.1 — Hindsight service (VM-3)**
- Postgres + pgvector container on VM-3 with the memory store schema (sessions → entries, vector embeddings, metadata). Separate from Temporal's Postgres (M0 note).
- **API contract** (the key deliverable — it is what "R/W Memory" in the diagram means concretely):
  - `read(session_key, scope)` → memory entries relevant to the current task/agent
  - `write(session_key, entry, embedding)` → persist a learning/artifact at task end
  - Scoping rules: what one task's writes are visible to later tasks (cross-task) vs. private to the run.
- Deliverable: API contract doc + working service + a client helper the executor containers use over the network.

**WP-4.2 — LLM-Wiki service (VM-2)**
- Content store for: design standards, development best practices, business best practices, tools/skills documentation (owner's model).
- Retrieval API: `query(topic) → relevant standards/skills passages` (embeddings or structured index; POC scale tolerates the simpler one — record the choice).
- **Ingest pipeline:** a documented curation path — where standards come from (repo / curated markdown), how they get indexed, and a version/cadence note (R7: who curates, how often). Even a manual "commit to repo → reindex" cadence is acceptable for POC, but it must be *documented*.

**WP-4.3 — Wire hydration into the lifecycle**
- M2's hydrate step now fans out to all three: LLM-Wiki query, Hindsight read, agent-catalog fetch (M3a's repo pull).
- At task end: Hindsight write (the "R/W" half) from the agent's report.
- Bounded hydration: record per-source latency; total hydration must stay within the M2 TTL headroom.

**WP-4.4 — Hydration test**
- One run through the M2 lifecycle showing: agent receives all three inputs (assert content present), writes a memory entry at task end, and a **second** task with related scope reads back what the first wrote (proves cross-task R/W).

## Deliverables

1. Hindsight service + **API contract doc** (read/write/scope semantics) + client helper.
2. LLM-Wiki service + retrieval API + ingest pipeline with a curation-cadence note.
3. Updated lifecycle hydrate/teardown steps (M2 config gains the three source endpoints + memory write-on-end).
4. Hydration test report (WP-4.4) including the cross-task read-back.

## Acceptance criteria (done-when)

1. **R/W over the network:** an ephemeral agent reads and writes Hindsight per session (WP-4.4, both directions).
2. **Cross-task:** task 2 sees task 1's memory write, per the scoping rules.
3. **Relevant retrieval:** LLM-Wiki returns the right standard/skill for a posed query (3 test queries, all correct).
4. **Catalog load:** agent boot still loads its full persona/skills from the repo with the other sources live (regression).
5. **Bounded:** total hydration time recorded and within M2 TTL headroom.

## Exit gate

M5 may start when criteria 1–5 pass. If the Hindsight API contract needs a fundamentally different shape (e.g. no per-session scoping), record the adaptation in the decision log — the diagram's "R/W Memory" only requires session-scoped read + end-of-task write for POC scale.

## Notes

- VM-2 and VM-3 are provisioned in M0; this milestone puts their **real** services on them.
- Keep content curation deliberately boring for the POC (markdown in a repo + reindex). R7's real question ("who curates, and what promotes from Hindsight into the LLM-Wiki?") is a Version 2 decision — this plan only builds the pipeline shape.
