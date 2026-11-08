# Chapter 10 — The Front Door

> **In plain terms.** Everything so far has been the back of house: how work
> is specified, executed, secured, and remembered. This chapter is the front:
> where the human's request enters, who understands it, who checks it, and
> how the answer comes back. It ends with the design question the whole
> project is moving toward — *should the greeter be a person or a program?*
> — and gives the answer the evidence supports.

## 10.1 The boundary that matters

The single most important architectural boundary in the system is the one
between the **thinking layer** and the **execution layer**:

- **Thinking layer** — the human and their assistant (a frontier LLM
  desktop, in the reference scenario). Free-form conversation. Research,
  refinement, questions, doubts, reformulations. Nothing here costs the
  pipeline anything, because nothing here is work yet.
- **Execution layer** — the pipeline. Structured work orders only. Durable,
  credentialed, audited, billed. Nothing here is ambiguous, because
  ambiguity cannot survive Chapter 4's strict schema.

The boundary exists for a reason the build demonstrated rather than
assumed: half-done thinking that leaks into execution produces failed tasks,
confusing reports, and wasted credentials. The front door's job is to make
the crossing *deliberate*.

## 10.2 The two doors, and who staffs them

The front-of-house machine hosts two small services:

**The gateway** — a thin, stateless receptionist. It accepts a request
(`title`, `capability`, `brief`, optionally an estimated deadline), answers
immediately with a task identifier and the assigned agent reference, and
forwards the request inward. It holds no state, no credentials, no logic
beyond validation. If the gateway dies mid-request, the requester simply
retries; nothing can be lost, because nothing has started.

**The registrar (the "manager")** — the deterministic matcher and composer
from Chapter 3. On each request it:

1. clones the roster and reads the machine index (Chapter 6);
2. matches the request's capability to a role — exact match first, then a
   deliberate, case-insensitive fallback;
3. composes the final work order: the role's behavioral specification plus
   the human's brief, merged into the self-sufficient envelope (Chapter 4);
4. publishes it to the pipeline;
5. later, correlates the result back and turns it into a **human report** —
   status, artifact pointer, and, on failure, a reason a person can read.

The report loop closes the circle: the human never talks to the durable
engine, the queue, or the workers directly — only to the gateway. That
single-path rule is what makes every task traceable and every door
auditable.

## 10.3 The design decisions and alternatives

**Decision: immediate acknowledgment, asynchronous result.** The gateway
answers in about a second with an ID; the work arrives later as a report.
Alternative: synchronous execution (request waits for result). Rejected:
long tasks would hold connections hostage, timeouts would kill healthy
work, and the human would be locked out of the thinking layer precisely
when they need it most. Asynchrony matches the mental model too: "send it
over" is a *handoff*, not a session.

**Decision: the registrar is code, not a model.** Matching capability to
role, validating the order, enforcing policy — these are lookup-and-verify
jobs. A deterministic registrar is fast (about a second to clone, match,
compose, publish), perfectly repeatable, and perfectly debuggable: when
something fails, the failure is *at a line*. The build's whole debugging
philosophy (Chapter 12) leans on this — every failure in a long, complex
system is cheaper to diagnose when part of the system never has an opinion.

**Decision: the strict schema is the door policy.** The registrar refuses
incomplete requests — loudly, with the specific missing field named.
The alternative — accept nearly-complete requests and fill gaps — was
explicitly tested by accident during the build, and the result (a task that
ran to completion on an incomplete order and *failed*) is Chapter 12's most
instructive story. Doors that guess are doors that leak.

**Decision: a single request is a single task — for now.** The current
registrar routes one request to one worker. Decomposition — one request
becoming three coordinated work packages — is the coordinator agent's job
(§10.4), and the pipeline already accepts the multi-package shape without
modification: nothing downstream knows or cares that two work orders came
from one conversation.

## 10.4 The open question this chapter exists to answer

The registrar's matching is deliberately mechanical: exact capability match,
then a tolerant fallback. For a small roster and structured requests, that
is *correct* — fast, free, and never wrong in a mysterious way.

But the roster is now forty-five roles across three departments, and the
human's request in the running example — *"market analysis on the
feasibility of applying stock market results against profit margin
historical data"* — is not a keyword. It is intent. Deciding that this
means *a feasibility specialist*, *a data architect*, and *a compliance
check* — or perhaps just one well-framed analysis task — is judgment.

The design answer, argued in Chapter 3 and reinforced by every reliability
lesson in Part III, is the **Chief of Staff**:

- a principal-layer agent (bus-org's coordinator, exactly as its roster
  role describes) that converses with the HITL, understands the request in
  business language, decomposes it into work packages, and for each
  *proposes* `{role, executor, specification, deadline}`;
- the existing gateway and registrar, unchanged, which *validate* each
  proposal against the roster and schema before anything executes;
- and the mechanical matching path retained as a **fallback**: when the
  coordinator is unavailable or the request already arrives structured, the
  front door still works exactly as it does today.

The principle, one line: **judgment proposes, code disposes.** The
coordinator adds understanding; the registrar refuses to add exceptions.

Two boundaries make this safe, and both are architectural rather than
aspirational:

1. **The coordinator can only speak the gateway's API.** It cannot publish
   to the pipeline directly, hold credentials, or spawn anything. Its
   proposals pass the same strict door every other request passes.
2. **The coordinator is long-lived and memory-backed** (Chapters 3 and 9) —
   the one agent in the system that *should* accumulate context, because
   coordination without continuity is just routing with extra steps.

## 10.5 What this buys

With the front door shaped this way:

- The HITL's experience is *one sentence in, one structured answer out* —
  regardless of how many workers the request eventually employs.
- Every delegation crosses exactly one audited boundary, carrying exactly
  one identity (the work order), returning exactly one artifact trail.
- The thinking layer stays cheap, safe, and human-paced; the execution
  layer stays durable, credentialed, and machine-paced — and neither ever
  impersonates the other.

Part II is complete at this point: roster, harnesses, credentials, memory,
front door — the five organs around the durable core. Part III now does
what a thesis must: it builds all of it, tests all of it, breaks pieces of
it on purpose, and reports exactly what happened — including everything
that went wrong and what each failure taught.
