# Store usage rules — what goes where (Hindsight, LLM-Wiki, git)

One-line rule for any artifact: **must be exact → git · must be recalled →
Hindsight · must be found → LLM-Wiki.** The sections below spell out each
store's exact use, what goes in, what never goes in, and why. These rules
consolidate D-stores, D-proj-id, D-wiki-truth, D-repo-truth, D-repo-ops,
D-catalog, and D-spec-source.

## Hindsight — long-term memory (vm-hindsight, shared, per project-id)

**Exact use:** the system's *recollection*. What the ephemeral agents
experienced, learned, and decided — so the next task doesn't start from zero.

Goes in:
- Session/task episodic memory: what was tried, what failed and why, what
  worked, decisions taken during the task.
- Explicitly promoted cross-task learnings (post-task reflections worth
  carrying forward) — written with the `cross-task` scope.
- Everything carries the **project-id** (D-proj-id): a project's memory is
  invisible to every other project.

Never goes in:
- Source code, documents of record, agent specs, standards — anything
  versionable or authoritative (those belong in git / the wiki).
- Anything anyone will ever need to *cite* as the truth. Hindsight has no
  review, no history, no PRs — it is memory, not storage of record.
- Anything another project should see (scope rules enforce this; TC-M4
  proves the scoping).

Retrieval is semantic (pgvector top-k): suited to "have we seen this before?"
— never the authority for a fact.

## LLM-Wiki — reference documentation (vm-llm-wiki, shared)

**Exact use:** the *consultable reference library*. Broad knowledge an agent
queries when relevant.

Goes in:
- Company/enterprise standards shared by all projects: design
  specifications, coding standards, architecture principles (D-wiki-truth).
- Per-project reference material, keyed by project-id: domain references,
  how-tos, runbooks, architecture overviews.
- Agent-design standards (D-spec-source): templates and norms for how the
  five agent markdowns are written — the standards catalog curation follows.

Never goes in:
- Agent operational specs (persona/skills/tools/behavioral) — catalog only
  (D-spec-source). The wiki describes how to design agents; it never holds
  the agents.
- Source code, deliverable documents, task outcomes — project repo territory.
- Memory/learnings — Hindsight territory.
- **Load-bearing standards stay out of wiki-only reliance**: anything an
  agent must *always* follow is pinned into that agent's behavioral-spec.md
  (hydrated byte-exact); the wiki serves the when-relevant long tail. When a
  wiki standard proves load-bearing for a class of agents, promote it into
  their behavioral specs.

Truth chain: the **wiki content repo** (git, byte-exact, reviewed) is the
source of truth; the pgvector database is a serving layer built from it by
the ingest job. Serving is probabilistic (top-k); correcting content means
editing the repo and re-running ingest — never patching the database.

## Git (the hosting platform — outside this architecture's operations, D-repo-ops)

**Exact use:** anything that must be *exact, versioned, reviewed, and
citable*. Files go in as whole documents; consumers check them out or load
them verbatim.

| Repo | Scope | Goes in | Never goes in |
|---|---|---|---|
| **Agent catalog** ("Available Agent List") | one, company-wide | agent specs only: index.json + per-agent folder (persona, skills, tools, behavioral-spec, constraints) — hydrated byte-exact at executor boot; manager resolves via index.json | reference docs (→ wiki), code, memory, anything not consumed at runtime |
| **Project repo** (GitHub/ADO) | one **per project** | source code + the project's versionable documentation (design docs, specs of record, deliverables); task branches `task/<task_id>` | reference material (→ wiki per the D-repo-truth boundary: deliverables → repo, reference → wiki), agent specs, memory |
| **Wiki content repo** | one, company-wide | the reference documents the wiki serves (enterprise + per-project reference) | anything not destined for wiki retrieval |

All three ride the same hosting platform; per project only the project repo
is added.

## The boundaries in one glance

| Artifact | Hindsight | LLM-Wiki | Git |
|---|---|---|---|
| Source code | — | — | project repo |
| Project docs of record | — | — | project repo |
| Enterprise standards (design specs, coding standards) | — | yes (content repo = truth) | wiki content repo holds them |
| Per-project reference material | — | yes (project-id area) | wiki content repo holds them |
| Agent-design standards (how to write specs) | — | yes (shared area) | wiki content repo holds them |
| Agent operational specs | — | **never** (D-spec-source) | agent catalog |
| Task memory: tried/failed/learned | yes (project-id scoped) | — | — |
| Task outcomes (branch + commit pointer) | pointer only | — | the artifact itself |
