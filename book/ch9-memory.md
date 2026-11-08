# Chapter 9 — Memory

> **In plain terms.** Chapter 1's first wall was continuity: agents that
> forget everything between tasks. The fix is not giving each worker a
> diary — it's giving the *organization* a filing cabinet. This chapter
> explains the memory system: what gets remembered, who may read it, and
> how a brand-new worker starts its task already knowing what the
> organization learned last week.

## 9.1 Memory belongs to the org, not the worker

Chapter 2's sixth principle (P6) draws the line precisely: **workers are
forgettable; the organization is not.** The implementation keeps that line
physical:

- A worker holds *nothing* between tasks — it is destroyed at task end
  (Chapter 5), so it *cannot* remember.
- Before it dies, it makes exactly **one write**: a small, structured entry
  — *what this task learned* — into the organization's memory store.
- The next worker, at boot, makes exactly **one read**: the entries relevant
  to its task, injected before its work begins.

The asymmetry is the design. A worker's memory of its own task dies with it;
what it *learned* survives it, as the org's property. Nothing personal, no
session state, no working context — only distilled, reusable knowledge.

## 9.2 The read contract: hydrate before work

At boot — alongside role hydration (Chapter 7) — the worker queries the
memory service: *what has this organization learned that is relevant to what
I am about to do?* The answer is injected into the task's context and, for
auditability, into the task's own log with a labeled marker.

Three properties make the read trustworthy:

1. **Scope separation.** Memory entries are tagged by scope. A task's
   private notes stay private to it; entries explicitly marked
   *cross-task* are what the organization shares. A worker reading memory
   sees the org's knowledge, not other workers' scratchpads.
2. **Degradation, never failure.** If the memory service is unreachable,
   the worker *proceeds without memory* — it logs the gap and continues.
   A memory outage must never stop work; it only makes work dumber. This
   is the same rollback discipline the roster (Chapter 6) and credentials
   (Chapter 8) follow: every auxiliary service is optional at read time,
   because the pipeline's guarantees come from the durable core, not from
   its satellites.
3. **Measured cost.** The read is milliseconds — the build measured
   single-digit-millisecond reads at task boot. Memory that costs real
   latency would be memory nobody hydrates.

## 9.3 The write contract: one learning per task

At task end — after the artifact is produced, before exit — the worker
writes its entry: a short summary of what the task learned, tagged with the
task's identity, its role, and the cross-task scope. Failure-tolerant by
design: if the write fails, the task's *result* still stands (the report
and artifact are already committed by the durable core); the org simply
loses that one lesson.

The discipline worth noting is what is *not* written: no transcripts, no
working state, no secrets, no data that could identify people or systems.
The memory holds the org's *conclusions about its work* — "this data source
had these quirks," "this approach failed for this reason" — the same thing
a good employee writes in the team handbook before leaving the project.
A memory full of everything is a memory nobody can use, and possibly one
nobody should be allowed to read.

## 9.4 How memory is stored — and what the embedding question is

Under the hood, the memory service pairs a conventional store with a vector
capability: entries can be embedded (turned into vectors) so that *relevant*
memories can be found by meaning, not just by keyword. This is the
mechanism that would let a risk analyst benefit from a data engineer's
learning — because the ideas are related even when the words are not.

The first build ran without an embedding provider: writes worked, reads
worked, and similarity search was deliberately degraded to exact and
scoped retrieval — with a numeric guard so the degraded path could not
produce nonsensical scores. This was an honest interim: the *contract*
(memory in, memory out, scope-separated) was fully proven; the *semantic
ranking* awaits a provider decision. The build's rule — prove the
contract, name the gap — applies here as everywhere: an incomplete
capability that is *labeled* is a plan; an incomplete capability that is
*hidden* is a defect.

## 9.5 Design decisions and alternatives

**Decision: read-and-write as worker contracts, not as infrastructure
magic.** The worker explicitly hydrates at boot and writes at end —
two small, auditable actions in its entrypoint. Alternative: transparent
memory attached to the model itself. Rejected: invisible memory is
unauditable memory, and it couples the org's knowledge to a specific
model runtime. Explicit contracts keep memory portable across harnesses
and inspectable by humans.

**Decision: writes are append-only and owned by the pipeline's identity
rules.** Workers cannot edit or delete each other's entries; curation —
deciding what becomes lasting institutional knowledge — is explicitly a
*human-owned* activity, logged in the knowledge repository the workers
also read. The system separates the fast layer (machine-written task
learnings) from the curated layer (human-curated permanent knowledge),
and never confuses them.

**Decision: degradation over dependency.** Every memory interaction is
failure-tolerant at both ends. The pipeline's completeness does not
depend on memory being up — which is what allows memory to be a separate
service on separate hardware, taken down for maintenance without a
maintenance window on the org itself.

## 9.6 What this buys

With memory in place, the organization exhibits the property that makes it
feel less like software and more like a workplace: **task N+1 is better
because task N happened.** A data quirk discovered by the first analyst
saves the second analyst an hour. A debugging insight from one engineer
changes how the next engineer approaches the same component. Nothing
except the *learning* is shared — no credentials, no session, no residue —
which is exactly the boundary a real organization draws between its
knowledge and its keys.

And because memory is explicit — read at boot, written at end, scoped,
logged — every claim in this chapter is checkable. The next worker's boot
log *shows* what it inherited. The proof-of-concept's evidence pack
includes that marker line, verbatim, from a worker that successfully
inherited another task's learning. In Chapter 11, that log line is the
exhibit.
