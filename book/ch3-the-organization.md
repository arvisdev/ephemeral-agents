# Chapter 3 — The Organization

> **In plain terms.** Chapter 2 stated six principles. This chapter shows the
> organization they produce: three departments, each with a distinct way of
> working; the route a piece of work takes through them; and the exact
> points where a human is in charge. If you understand this chapter, you
> understand the system's shape — everything after this is detail.

## 3.1 Three departments, three ways of working

The organization has three departments. The distinction between them is not
topic area — it is **how they work**, which drives *where they run* and
*when they are used*.

### The Business Organization (bus-org) — judgment, early and late

Twelve roles that do the work a company's business side does: a Product
Manager who owns intent, a Requirements Analyst who captures needs, a
Stakeholder Analyst who maps who is affected, a Risk Assessment Analyst who
quantifies what could go wrong, a Quality Assurance Specialist who verifies
results, a Domain Compliance Specialist who proves obligations are met, a
Chief of Staff who coordinates and gates.

These are **principals**: long-lived, conversational, judgment-heavy. They
work *with the human* — in dialogue, iteratively, before and after execution.
They never run as disposable containers, because their value is continuity:
they hold the thread of a decision across weeks. They live in the thinking
layer — the HITL's desktop and, in the target architecture, the front-of-house
machine that receives work.

### The Engineering Organization (ea-org) — design, in the middle

Eleven roles that turn "what we want" into "how to build it": an Enterprise
Architect who decomposes, a Solution Architect who specifies, a Data
Architect who models, a Security Architect who threat-models, a UI/UX
Designer who designs experience — plus senior engineers (backend, full-stack,
data, test, GitHub operations) who define what good execution looks like.

The architects are principals like the business org. But ea-org has a second
job that matters more than its members: it defines the **work-package
standard** — the format that makes a task self-contained enough for a
one-shot worker to execute without asking questions. Every time the
engineering org improves a work package, every future worker benefits.

### The Ephemeral Organization (ephemeral-org) — execution, at the middle

Twenty-two workers, each mirroring one of the roles above in scoped,
single-task form: `eph-backend-engineer`, `eph-risk-assessment-analyst`,
`eph-unit-test-engineer`, and so on. They are the shop floor. A worker:

1. is **created** for exactly one work order,
2. **hydrates** the persona, skills, and behavioral spec of its role,
3. **works** — produces the requested artifact,
4. **reports** what it produced, assumed, and left open,
5. and **ceases to exist**, leaving nothing behind but its artifact and a
   memory note.

They have no meetings, no memory of each other, and no access beyond their
one task. That is not a limitation — it is the mechanism that makes the
organization safe to scale.

## 3.2 The life of a piece of work

Work moves through four stages. Each stage names the department in charge —
and, crucially, the *human's* position in it.

```
STAGE 1 · INTENT                    (bus-org, with the HITL)
  "Should we look at applying stock market results to our margin data?"
  → requirements captured, prioritized, made testable

STAGE 2 · SPECIFICATION             (bus-org → ea-org, gated by the CoS)
  → spec-ready requirements, behavior specifications
  → architecture, work packages in the standard format

STAGE 3 · EXECUTION                 (ephemeral-org, the pipeline)
  → each work package dispatched to one eph-worker
  → artifact produced, verified against the package's acceptance criteria

STAGE 4 · VERIFICATION & GATE       (bus-org, back with the HITL)
  → QA verifies with evidence; compliance checks obligations
  → Chief of Staff rolls up and gates release — or re-dispatches findings
```

The loop between Stage 3 and Stage 4 is deliberate: quality findings are not
complaints, they are **new work packages** fed back into execution. The same
pipeline that does the work also absorbs its own corrections.

## 3.3 Where the human sits

The design gives the human — the Human-In-The-Loop (HITL) — exactly three
positions of authority, and resists giving it more:

1. **The conversation.** All intent originates with the HITL, refined in
   dialogue with the bus-org principals. The HITL never writes work orders by
   hand; the organization turns understanding into orders.
2. **The handoff.** "Send it over" is a human decision. Work does not begin
   executing because a model decided to; it begins because a person sent it.
   This is the system's single most important safety property.
3. **The gate.** Verification results come back to the HITL with evidence.
   Acceptance is a human act, supported by the QA specialist's proof — not
   replaced by it.

Everything between those three points is the organization working on its own.
That is what makes the system delegable: the human's involvement is
concentrated where judgment is irreplaceable, and absent where it would only
add latency.

## 3.4 The routing question — and its answer

A natural question: when the HITL says "send it over," *who decides which
department and which worker?*

The design answers with the division established in Chapter 2 (P1): **an
agent proposes, code disposes.** The front-of-house has two layers:

- **A coordinator agent** — the bus-org Chief of Staff — that understands the
  request in natural language, decomposes it into work packages, and, for
  each, *proposes* the specialist from the roster, the executor type, and the
  specification. Judgment lives here, reading the roster the way a manager
  reads a staff directory.
- **A deterministic registrar** — plain code — that *validates* every
  proposal before anything executes: does this role exist in the roster? Is
  the work order complete and schema-valid? Is the executor authorized? What
  TTL applies? Only a verified work order enters the pipeline.

The registrar is deliberately boring. It cannot be convinced, it does not
improvise, and its refusals are exact. Everything the organization learned
about reliability during its build (Part III) argues for this split — and
against two failure modes in particular: a registrar that routes by keyword
(scales badly, misunderstands always) and a coordinator that is also the
registrar (judgment without a check).

## 3.5 The shape, in one picture

```
        H I T L   (you, at your desk)
              │  conversation            ▲ reports back
              ▼                          │
   ┌─────────────────────────────────────────────┐
   │  PRINCIPALS — long-lived, judgment          │   thinking layer
   │  bus-org (12 roles) · ea-org architects     │
   │  incl. the CHIEF OF STAFF — intake,         │
   │  decomposition, dispatch proposals          │
   └───────────────┬─────────────────────────────┘
                   │ work orders (validated by the registrar)
                   ▼
   ┌─────────────────────────────────────────────┐
   │  THE PIPELINE — durable, deterministic      │   execution layer
   │  work orders · credentials · scheduling     │
   └───────────────┬─────────────────────────────┘
                   ▼
   ┌─────────────────────────────────────────────┐
   │  WORKERS — ephemeral, single-task           │
   │  ephemeral-org (22 roles), one task each    │
   │  hydrate → work → report → gone             │
   └─────────────────────────────────────────────┘
```

Part II now builds this picture component by component — and for each one,
shows not just what it is, but the decisions and alternatives behind it.
