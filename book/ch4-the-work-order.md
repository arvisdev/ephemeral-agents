# Chapter 4 — The Work Order

> **In plain terms.** Every piece of work in the system travels as a small,
> structured document called a *work order* — like a job ticket, but one that
> machines can verify. This chapter explains what's in it, why each field
> exists, and why the discipline of "everything the worker needs must be in
> the envelope" turned out to be the single most important design rule of the
> whole project.

## 4.1 What a work order is

When a work package leaves the principals' layer and enters the pipeline, it
is reduced to a fixed-shape record — versioned, small (under a kilobyte),
and readable by both machines and people:

```json
{
  "task_id":        "024450cf-d265-4d25-92b1-aa99f56dda52",
  "executor":       "hermes",
  "agent_ref":      "https://github.com/arvisdev/agent-roster.git#ephemeral-org/eph-risk-assessment-analyst",
  "repo_folder":    "ephemeral-org/eph-risk-assessment-analyst",
  "repo_target":    "repos/ephemeral-poc-demo",
  "task_prompt":    "Assess risks of the proposed analysis approach…",
  "max_turns":      20,
  "behavioral_spec": "<the role's spec + the task brief>",
  "requester":      "desktop-engineer",
  "ttl_s":          900
}
```

Eleven fields. Each one exists because its absence was once a failure.

## 4.2 The fields, and why each earns its place

**`task_id` — the single name of the work.** One opaque identifier used
everywhere: in the durable engine, in logs, in the report, in the memory
entry. No separate "correlation id," no session token — one name, end to end.
This was a deliberate simplification: every attempt to add a second
identifier created a way for two records to disagree.

**`agent_ref` — who does the work.** A pointer into the roster:
`<repo url>#<folder>`. The `#folder` convention splits at one character and
gives the worker exactly what it needs: the repository to clone and the
folder to hydrate from. Before this contract existed, workers received the
two halves in different shapes from different components — and silently
misassembled them.

**`executor` — which harness runs the role.** Two executor images exist
(Chapter 7): a general agent harness and a code-editing harness. The field
makes harness choice an explicit, per-task decision rather than a global
assumption — and when it is absent, the pipeline applies a configured
default, with a deliberately un-glamorous rollback option (a no-op stub)
preserved as the safety pin.

**`behavioral_spec` — the full instructions.** This field carries *the role's
specification plus the task brief*, composed by the registrar before
dispatch. The rule it enforces is the chapter's title: **the envelope is
self-sufficient.** A worker must never need to ask a follow-up question,
because there is no one to ask — its coordinator is gone the moment the
order is published. Early in the build, briefs and specs traveled separately
and workers executed on fragments; merging them into one field closed an
entire class of "the agent only saw the first word of its instructions"
failures.

**`ttl_s` — the deadline.** Every work order carries its own expiration.
The executor container is *born with its death certificate*: a time label the
reaper reads. If the worker hangs, wanders, or the world ends around it, the
deadline reclaims the machine. Two-grace design (worker grace before the
reaper may act) keeps the deadline from killing a healthy, slow worker.

**`max_turns` — a second clock.** An LLM worker can fail productively —
looping, second-guessing, never converging — without ever hitting its wall
clock. The turn budget bounds *reasoning effort*, not just time.

**`repo_target`, `requester` — context and accountability.** Where the work
belongs, and who asked. The requester field is small but non-negotiable: it
is what makes the audit trail answer "who ordered this?" without guesswork.

## 4.3 The design decisions, and the alternatives rejected

**Decision: one fixed schema, versioned by filename.** Alternatives: free-form
payloads (rejected: nothing downstream can validate), per-task-type schemas
(rejected: the pipeline must treat all work identically to stay simple), or
versionless evolution (rejected: the filename carries the version so old and
new orders can coexist).

**Decision: the registrar composes the final instructions.** Early designs let
the requester attach instructions directly. That split authority: two writers
per order, no one owning completeness. Moving composition into the registrar
(the "Note-1 job") made the order single-author — and single-author orders
are the ones that arrive complete.

**Decision: `additionalProperties: false` — strict schema.** Unknown fields
are rejected, not ignored. This felt pedantic until the first schema-drift
bug: a component silently adding a field it believed essential, another
silently depending on it, and neither documented anywhere. Strictness turned
that drift from a latent bug into an immediate, loud error.

**Decision: no field for "expected output format."** The output contract
lives in the role's behavioral specification (Chapter 6), not in the order.
Rationale: the *role* defines what its artifact looks like — that knowledge
belongs in the roster, next to the skills, not duplicated per request.

## 4.4 The rule this chapter teaches

Everything else in the system is built to respect one property of the work
order: **it is complete, or it is rejected.** The pipeline never "helps"
an incomplete order by guessing. It refuses, loudly, with a reason the human
can read. Half the reliability of the system comes from machines doing
exactly this — refusing politely — instead of improvising around gaps.

That discipline was not free; it was learned. Part III tells the story of the
run where a near-complete order was *accepted* by every component and still
produced a failed task — and how "the gate must assert the outcome, not the
absence of debris" became a permanent rule.
