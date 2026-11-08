# Chapter 5 — The Durable Core

> **In plain terms.** Chapter 4 described the work order — the ticket. This
> chapter is about the *mill* that executes tickets: the machinery that picks
> them up, keeps them safe across failures, spawns the worker, watches the
> clock, cleans up, and reports the result. Its defining property: a task,
> once accepted, **cannot be lost** — no crash, restart, or forgotten deadline
> makes work vanish.

## 5.1 The problem the core exists to solve

An LLM worker is a process on a machine. Machines reboot, networks partition,
containers get killed, and — the most human failure of all — someone shuts a
VM down at 10 p.m. without checking. If the work "lives in" the running
process, every one of those events destroys it.

The core's job is to make sure work never lives in a process. Work lives in a
**durable record**; processes are merely how the record gets advanced. If a
process dies, another picks up the record and continues. This is the same
insight that underlies every serious order-processing system, and the design
borrows it wholesale rather than inventing it.

## 5.2 The components

Three services share the core's responsibilities, each deliberately
single-purpose:

**The workflow engine (durable state machine).** Every work order becomes one
*workflow instance* — a recorded, resumable sequence of steps: *dispatch →
spawn worker → poll until terminal → reap → publish result*. The engine
persists every step's completion. If the host dies after "spawn worker" but
before "poll," the instance is still there, and resumes exactly there. Work
orders are matched to instances by `task_id`, the single name from Chapter 4.

**The dispatcher (queue).** Work orders travel between machines on a
message bus — a *topic exchange* so that any component interested in
`agent.tasks.*` can subscribe without the publisher knowing it exists. The
queue is durable: a message survives until a consumer confirms it. This is
what allows the front-of-house machine and the execution machine to be
different VMs — and to reboot independently.

**The lifecycle service (worker management).** This is the component that
touches real machines, and it owns four verbs:

- **spawn** — create the worker container, wired with its work order's env
  (role, spec, credentials), attached to the right network, labeled with its
  `task_id` and its *expiration timestamp*;
- **poll** — watch the worker's state on a short interval;
- **reap** — destroy the worker when it exits (or when its TTL expires), and
  *verify zero survivors* afterward;
- **report** — publish the outcome back onto the bus for the front-of-house.

## 5.3 The two clocks

A worker can fail two ways, and they need two different clocks:

**The wall clock (`ttl_s`).** Set at spawn, stamped on the container as a
label. The reaper enforces it: when the deadline passes, the worker dies —
by its own exit if it finished, by the reaper if it didn't. The deadline is
*carried by the work order* (Chapter 4), so even a crash of the scheduler
does not lose it; the reaper re-reads it from the container itself.

**The reasoning clock (`max_turns`).** Wall time says nothing about whether
a worker is *converging*. A worker can burn an hour making no progress, or
finish a subtle task in ninety seconds. The turn budget bounds the number of
reasoning steps, complementing the deadline.

Both clocks end the same way: the worker exits (code 0 = success, anything
else = failure), the lifecycle records the exit code, and the outcome —
success, failed, or timed-out — travels back as the report.

## 5.4 The reaper, and the zero-survivors guarantee

The reaper is the component that makes Chapter 2's P4 (ephemeral workers)
true rather than aspirational. Its contract:

1. A worker container is labeled at birth with its task and its expiry.
2. A background reaper sweeps continuously, destroying anything past expiry —
   regardless of which component was supposed to have handled it.
3. After any task ends, the system *asserts zero survivors*: a count of
   containers bearing the worker label. Zero is the pass condition; anything
   else is a failure of the system itself, not of the task.

The "zero survivors" check deserves emphasis because it inverts a common
pattern. Most systems ask "did the task succeed?" and separately, maybe,
"did anything get left behind?" This system makes *leftovers a first-class
failure*. The result, measured across the entire proof-of-concept build:
**every single completed run — including deliberate failure injections and a
mid-task service restart — ended with zero surviving containers.**

## 5.5 Design decisions and alternatives

**Decision: a workflow engine rather than queues-with-retries.**
Alternative: pure message queues, where each step is a message and failure
handling is redelivery. Rejected because the sequence has *order and
preconditions* (don't reap before spawning; don't report before the outcome
is known), and encoding that order in queue semantics scattered the logic
across consumers. The engine keeps the sequence as one inspectable record —
which is also what makes the audit trail of Chapter 2 possible: the
*history of a task* is a single artifact.

**Decision: labels over databases for worker bookkeeping.** Worker identity,
task, and expiry live on the container itself as labels. Alternative: a
database of "known workers." Rejected: two sources of truth disagree — the
classic drift that produced several of the build's failures (Part III). The
container *is* its own record; if it exists, the reaper can find it; if it's
gone, nothing needs cleanup.

**Decision: the worker's exit code is the truth.** No component second-guesses
a worker's self-reported success. If the worker says 0, the task completed;
if it says otherwise, the task failed — and the *report* carries the reason.
One lesson from the build shaped this hard: a component once "helpfully"
treated a cleanly-reaped failed task as a success because cleanup had gone
well. Cleanup is not success. The gate must assert the outcome.

**Decision: single engine, honest about the trade.** One workflow engine is a
central point — the design accepts that for now. The alternative (multiple
independent engines per region/department) is deferred until scale demands
it, per the V2 open-items list (Chapter 13). Correctness first; distribution
is a later, well-understood problem.

## 5.6 What this buys

With the core in place, the *entire rest of the system* gets to be simple:

- The front-of-house never tracks anything — it publishes and waits for a
  report (Chapter 10).
- Workers can be as stateless and disposable as Chapter 6 requires — because
  the *system* remembers, not them.
- Failure handling is not a feature of each component; it is a property of
  the one engine that owns the record.

The remaining chapters add intelligence to this skeleton: the roster that
defines roles (Chapter 6), the harnesses that animate them (Chapter 7), the
credentials that make them safe (Chapter 8), and the memory that makes the
organization smarter over time (Chapter 9).
