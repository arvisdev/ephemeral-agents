# Available Agent List — repository contract

**Decision (D-catalog, 2026-10-30):** the agent specifications live in **their
own GitHub repository** — the *Available Agent List repo* — not inside this
POC repo. The diagram places it inside the company boundary but outside the
POC system: it is shared agent infrastructure that V2 will consume unchanged.
Runtime consumers clone it (`git clone --depth 1`) with a dedicated read-only
token (owner handover item; see `deploy/README.md` "Owner must hand you
these"): the **M3 executors** hydrate persona/skill/tool markdown at container
boot, and the **M5 manager** clones it to resolve the agent for a request.
Separate repo ⇒ separate least-privilege credential (FR-S1/S2) and an
independent curation cadence (R7: curation ownership is a recorded V2
open item).

## Layout contract (what the manager and executors rely on)

```
<repo root>
├── index.json                     # the catalog index the manager searches
└── <agent-folder>/                # one folder per agent (name = folder name)
    ├── persona.md                 # who the agent is
    ├── skills.md                  # what it can do (capability list)
    ├── tools.md                   # tool access granted
    ├── behavioral-spec.md         # the Behavioral Specification (Note 1)
    └── constraints.md             # guardrails / non-goals
```

### `index.json`

A JSON array; one entry per agent:

```json
[
  {
    "ref": "agents/backend-coder",
    "name": "Backend Coder",
    "role": "backend-coder",
    "folder": "agents/backend-coder",
    "skills": ["go", "rest-apis", "postgres"],
    "spec_ref": "agents/backend-coder/behavioral-spec.md"
  }
]
```

Field rules:
- `role` is what the manager matches against the request's
  `agent_capability`; keep one canonical role string per agent.
- `folder` is the path inside THIS repo (the manager turns `agent_ref` =
  `<repo>#<folder>`; executors receive the folder and load every markdown
  in it).
- `spec_ref` points at the Behavioral Specification; if the file is named
  `behavioral-spec.md` inside the folder, the manager uses it without
  consulting `spec_ref` — `spec_ref` exists for agents whose spec lives
  elsewhere (shared or out-of-folder).
- `skills` is advisory (surfaced in reports); **do not** add fields the
  manager does not read — unknown keys are ignored by design, not contract.

### Example catalog

A minimal but working example lives at `deploy/m5/agent-list-example/` in
this repo: copy it into a fresh GitHub repository (private), add the repo URL
+ a read-only token to the owner-inputs (`AGENT_LIST_REPO` + token), and the
M5 E2E run can resolve and dispatch against it.

## Change rules

- The catalog is **owner-curated** (R7). The POC's scripts and services never
  write to it; only the owner (or a future curation flow) does.
- Never rename an agent's `folder` while a task referencing it is in flight —
  executors hydrate by path.
- Contract changes to `index.json`'s shape belong in this document first,
  then the manager (`services/manager/`) — the shape above is what the M5
  gate (TC-M5-01) exercises.
