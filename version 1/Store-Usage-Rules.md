# Store usage rules — what goes where (Hindsight, llm-wiki git repo)

One-line rule for any artifact: **must be exact → git · must be recalled →
Hindsight · must be found → the llm-wiki git repo.** The sections below spell
out each store's exact use, what goes in, what never goes in, and why. These
rules consolidate D-stores, D-proj-id, D-wiki-truth, D-repo-truth, D-repo-ops,
D-catalog, D-spec-source, and D-wiki-git (2026-11-04: the LLM-Wiki RAG service
was removed; the wiki is now the plain git repo `github.com/arvisdev/llm-wiki`,
consumed by clone — D-wiki-truth is now trivially true: the content repo IS the
whole store, no serving layer).

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
  versionable or authoritative (those belong in git / the llm-wiki repo).
- Anything anyone will ever need to *cite* as the truth. Hindsight has no
  review, no history, no PRs — it is memory, not storage of record.
- Anything another project should see (scope rules enforce this; TC-M4
  proves the scoping).

Retrieval is semantic (pgvector top-k): suited to "have we seen this before?"
— never the authority for a fact.

## llm-wiki — reference documentation (git repo, shared)

**Exact use:** the *consultable reference library*. Broad knowledge an agent
consults when relevant. As of D-wiki-git (2026-11-04) this is a plain git
repository — **https://github.com/arvisdev/llm-wiki** (private) — not a
serving service: consumers shallow-clone it (`git clone --depth 1`) and read
the markdown directly. There is no retrieval API and no database.

Goes in:
- Company/enterprise standards shared by all projects: design
  specifications, coding standards, architecture principles (D-wiki-truth).
- Per-project reference material, keyed by project-id: domain references,
  how-tos, runbooks, architecture overviews.
- Agent-design standards (D-spec-source): templates and norms for how the
  five agent markdowns are written — the standards catalog curation follows.

Never goes in:
- Agent operational specs (persona/skills/tools/behavioral) — catalog only
  (D-spec-source). The repo describes how to design agents; it never holds
  the agents.
- Source code, deliverable documents, task outcomes — project repo territory.
- Memory/learnings — Hindsight territory.
- **Load-bearing standards stay out of wiki-only reliance**: anything an
  agent must *always* follow is pinned into that agent's behavioral-spec.md
  (hydrated byte-exact); the repo serves the when-relevant long tail. When a
  standard proves load-bearing for a class of agents, promote it into
  their behavioral specs.

Truth chain: **the repo itself** (git, byte-exact, reviewed) is the source of
truth — there is no serving layer to keep in sync (D-wiki-truth, now
trivially). Correcting content means editing the repo; every clone made after
the push sees it. Interim hydration default: a full shallow clone at executor
boot (see the V2 open item on hydration/retrieval shape — a curated
index/README-per-topic retrieval aid may be added later). Curation cadence
and ownership are tracked in the repo's CHANGELOG.md.

## Git (the hosting platform — outside this architecture's operations, D-repo-ops)

**Exact use:** anything that must be *exact, versioned, reviewed, and
citable*. Files go in as whole documents; consumers check them out or load
them verbatim.

| Repo | Scope | Goes in | Never goes in |
|---|---|---|---|
| **Agent catalog** ("Available Agent List") | one, company-wide | agent specs only: index.json + per-agent folder (persona, skills, tools, behavioral-spec, constraints) — hydrated byte-exact at executor boot; manager resolves via index.json | reference docs (→ llm-wiki), code, memory, anything not consumed at runtime |
| **Project repo** (GitHub/ADO) | one **per project** | source code + the project's versionable documentation (design docs, specs of record, deliverables); task branches `task/<task_id>` | reference material (→ llm-wiki per the D-repo-truth boundary: deliverables → repo, reference → llm-wiki), agent specs, memory |
| **llm-wiki** (github.com/arvisdev/llm-wiki) | one, company-wide, private | the reference documents agents consult (enterprise + per-project reference): `design-standards/`, `dev-best-practices/`, `business-best-practices/`, `tools-and-skills/`, `CHANGELOG.md` | anything not destined for reference lookup; agent operational specs |

All three ride the same hosting platform; per project only the project repo
is added.

## The boundaries in one glance

| Artifact | Hindsight | llm-wiki (git repo) | Git |
|---|---|---|---|
| Source code | — | — | project repo |
| Project docs of record | — | — | project repo |
| Enterprise standards (design specs, coding standards) | — | yes (the repo = truth) | the llm-wiki repo holds them |
| Per-project reference material | — | yes (project-id area) | the llm-wiki repo holds them |
| Agent-design standards (how to write specs) | — | yes (shared area) | the llm-wiki repo holds them |
| Agent operational specs | — | **never** (D-spec-source) | agent catalog |
| Task memory: tried/failed/learned | yes (project-id scoped) | — | — |
| Task outcomes (branch + commit pointer) | pointer only | — | the artifact itself |
