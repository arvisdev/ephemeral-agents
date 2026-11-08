# Chapter 6 — The Roster

> **In plain terms.** A company is only as good as its job descriptions. This
> chapter explains the *roster* — the organization's staff directory — where
> every role is written down: what the person is like, what they can do, and
> exactly how they behave on the job. The roster is not documentation about
> the system; it *is* the system's talent. When a worker is spawned, it
> literally becomes one of these roles by reading its job file.

## 6.1 Why roles must be written, not assumed

Chapter 2's second principle (P2) says specialization beats generalization.
But specialization is only real if it is *specified*. "A good analyst" is not
a role; "an analyst who structures requirements into identified, owned,
testable items, maintains traceability, and flags ambiguity before it
propagates" is a role.

The roster therefore stores, for every role, a small set of files that
together constitute the job description:

| File | What it holds |
|---|---|
| `persona.md` | The professional archetype: experience, communication style, behavioral traits |
| `skill-and-tools.md` | The craft (what the role can do) and the instruments (what it may use) |
| `behavioral-spec.md` | The operating rules: how this role behaves on a task, its work pattern, its completion contract |

Three files per role, no more. The constraint is deliberate: a job
description that takes an hour to read will never be maintained; one that
fits on a screen or two gets refined every time the organization learns
something.

## 6.2 Three departments, forty-five roles

The roster is organized as three departments (Chapter 3):

- **bus-org** (12 roles) — the business principals: product management,
  requirements, stakeholders, risk, compliance, domain expertise, process,
  quality, feasibility, coordination.
- **ea-org** (11 roles) — the engineering principals and standards: enterprise
  and solution architecture, data, security, design, and the senior
  engineering crafts.
- **ephemeral-org** (22 roles) — the spooler workforce: the `eph-` twins of
  the principal roles, each defined for single-task execution.

The eph- workers deserve a closer look, because they are the roles the
pipeline executes. Each eph- role carries, in its behavioral spec, the
*single-task contract* that Chapter 3 described:

> Receive exactly one work package; do only the work it describes. Produce
> the requested artifact(s). Write the artifact and a concise completion
> report, then exit. Do not start adjacent work; flag it instead. On
> failure: report the reason plainly and exit non-zero.

Every worker in the department shares this skeleton; what differs is the
craft on top of it. That commonality is what lets the pipeline treat all
work identically (Chapter 4's fixed schema) while the *content* of work
varies infinitely.

## 6.3 The index: making the roster machine-readable

A staff directory is only useful if the registrar can search it. Alongside
the role files, the roster carries a machine-readable **index** — one entry
per executable role:

```json
{
  "ref":      "ephemeral-org/eph-risk-assessment-analyst",
  "name":     "Risk Assessment Analyst",
  "role":     "risk assessment analyst",
  "folder":   "ephemeral-org/eph-risk-assessment-analyst",
  "skills":   ["risk register", "likelihood × impact sizing", "…"],
  "spec_ref": "ephemeral-org/eph-risk-assessment-analyst/behavioral-spec.md"
}
```

Each field has one consumer:

- `role` and `ref` are what the **registrar** matches a request's capability
  against (Chapter 10);
- `folder` is what the **worker** hydrates from (the `#folder` half of the
  work order's `agent_ref`, Chapter 4);
- `skills` are human-facing keywords and machine-facing hints for richer
  matching later;
- `spec_ref` is the fallback instruction pointer when the spec file itself
  cannot be read.

The index accepts two shapes — a bare array and a wrapped object — because
during the build the catalog's real shape differed from the documented one,
and a registrar that accepts both shapes refuses to be the reason work
stops (that story is told properly in Chapter 12).

## 6.4 How a role is born

Writing a role is a writing task, not a coding task — which is itself a
design point: **the roster is editable by people who are not programmers.**
A new role appears by adding a folder with the three files and one index
line. The pipeline needs no change, no redeploy, no registration step.

The quality bar for a role definition, learned by doing:

1. **The persona states how the role thinks and communicates**, not just
   what it knows. ("Verdict-first, evidence-second" is behavior; "knows
   risk analysis" is a keyword.)
2. **The skills name the craft's instruments**, so the worker both knows
   what it may reach for and what is out of scope.
3. **The behavioral spec ends in a completion contract** — the exact
   definition of done. This is the sentence the whole system leans on:
   the worker's report, the registrar's gate, and the HITL's acceptance
   all refer to it.
4. **House jargon is banned.** Early role files drifted into internal
   shorthand that no newcomer (human or machine) could act on. The rule
   that stuck: every role file must be readable and actionable by someone
   who has never met the organization.

## 6.5 Design decisions and alternatives

**Decision: files in a repository, not rows in a database.** The roster is
plain markdown in a version-controlled repository. Alternatives: a role
database (rejected: loses version history, review workflow, and human
editability), or per-role containers (rejected: conflates the *role* with
the *harness* — Chapter 7's separation). A repository gives free history,
diffs, review, and the same clone-at-runtime property the workers need.

**Decision: personas are prose, not parameters.** Alternative: structured
role definitions (JSON of capabilities and limits). Rejected after early
experiments: structured definitions produced workers that obeyed the letter
and missed the spirit. Prose personas — written the way a good manager
describes a good hire — hydrated into noticeably better role behavior. The
index carries the structured minimum; the persona carries the judgment.

**Decision: one roster serves two consumers.** The same repository is both
the *human* staff directory (with a formatted roster document per department,
describing every role for selection) and the *machine* catalog (index.json +
hydration files). One source of truth; two renderings. The alternative —
keeping a "documentation copy" and a "runtime copy" — is exactly the kind of
drift the build's lessons forbid: nothing load-bearing may live in two
places (Chapter 12, Lesson L4).

**Decision: the roster is the routing surface.** The registrar matches
request capabilities against `role`/`ref` strings. This makes roster
authoring a *routing* decision: writing "risk assessment analyst" as a role
is what makes a request about risk findable. The alternative — a separately
maintained routing table mapping keywords to roles — was rejected as one
more thing to keep in sync; the roster's job titles *are* the routing table.

## 6.6 What the roster does not do

It does not decide *who* is picked for a task (that is the coordinator's
judgment, Chapter 10); it does not define *how* a worker executes (that is
the harness, Chapter 7); and it holds no secrets — the roster is, by
design, the most public artifact in the system, because a job description
contains nothing that needs protection.

The roster is the organization's promise to itself: *every role we can
staff is written down well enough that a stranger — human or machine —
could do the job from the file alone.* Part III will show why that promise,
kept, is what let a one-shot container behave like a fifteen-year
specialist.
