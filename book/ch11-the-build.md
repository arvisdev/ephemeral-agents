# Chapter 11 — The Build

> **In plain terms.** Parts I and II described an organization of AI agents
> on paper. This chapter tells the story of building it for real — six
> construction stages over four days, on real virtual machines, with real
> credentials, real failures, and a rule that nothing is "done" until a test
> proves it. Nothing in this chapter is a plan; all of it is a record.

## 11.1 The rules the build was run under

Before the first machine was provisioned, the build adopted rules that
shaped everything that followed:

1. **Real infrastructure only.** Every claim had to be demonstrated on real
   virtual machines with real credentials — no mocks standing in for the
   parts that matter (credentials, durability, cleanup).
2. **Gates, not vibes.** Each construction stage ended in a *test procedure*
   with named cases. A stage was complete only when every case passed and
   the results were formally signed. No partial credit; a stage that failed
   one case stayed open until it didn't.
3. **Root causes, not patches.** Every failure was diagnosed to its cause
   before the next attempt. Roughly thirty-five distinct defects were found
   and fixed across the build; each one's story is ledgered (Chapter 12).
4. **All test inputs live in the repository.** Any rerun must be able to
   reproduce the test from committed material — a rule that earned its keep
   repeatedly (and whose violation, once, became a lesson of its own).
5. **Evidence is filed, not narrated.** Every gate's results, logs, and
   artifacts were filed with the build; the narrative in this chapter can
   be checked against it.

## 11.2 The six stages

### Stage 0 — The chassis (M0)

Virtual machines provisioned to a baseline: container runtime installed and
verified on each, a message broker stood up, a workflow engine healthy with
its datastore. The stage's gate proved the *ground* the rest would build
on — and produced the first thesis-grade evidence: the chassis checks are
idempotent, meaning the whole stage can be re-run against a living estate
without rebuilding it.

### Stage 1 — The message bus (M1)

The topic exchange and the durable work queue were stood up and proven with
a dedicated bridge consumer. The gate included the first *cross-machine*
proof: a message published on one machine, consumed on another — the
decoupling that lets the front-of-house and the workers live apart.

### Stage 2 — The durable core (M2)

The workflow engine's task workflow, the lifecycle service, the worker, and
the reaper — Chapter 5's machinery — were built and gated. This stage fixed
the most defects of any (fourteen ledgered), which is what one should
expect: it is the stage where the *contracts between components* were first
exercised. Its signature proofs: crash-recovery (a workflow's host killed
mid-task; work resumed), TTL expiry (a worker past its deadline reclaimed
cleanly), and the mixed-run test (success, failure, and timeout paths all
producing correct outcomes).

### Stage 3 — The roster and the executors (M3)

The agent catalog, the two executor harnesses, and the credential issuer
came online. The stage's gates proved the thesis's most distinctive claims:

- **Persona correctness:** a worker hydrated a role's full persona
  (three files, sub-second boot) and self-reported it verifiably.
- **Failure paths:** workers instructed to fail, failed *cleanly* — proper
  exit codes, complete reports, zero residue.
- **Real work:** the code executor performed a genuine repository edit —
  clone, branch, change, push — driven by the roster role and the work
  order, through the full pipeline.
- **Credential lifecycle:** tokens minted per task, and *proven dead* after
  it — the denial verified, repeatedly.
- **Concurrency:** the chosen model (one worker per task, per-task branches)
  exercised sequentially with no interference.
- **Secret hygiene:** every image scanned; zero secret findings.

### Stage 4 — Memory (M4)

The memory service and its contracts went live: read at boot, write at end,
scope separation, and cross-task inheritance — a later task demonstrably
receiving an earlier task's learning in its boot context (the exhibit
Chapter 9 promised). Gates covered in-container access, cross-task recall,
knowledge-repository lookups, catalog regression, latency, a full curation
cycle, and storage isolation.

### Stage 5 — The front door and the whole system (M5)

The gateway and registrar went live, and the build's final gates ran
*against the entire assembled system*:

- **Stage-gated bring-up**, seven stages, all green;
- **The end-to-end acceptance run:** one human request, human to human —
  request accepted, durable workflow executed, worker performed real work
  (a genuine repository commit, verified on a fresh clone), report returned,
  **zero surviving containers**;
- **Failure injections, five ways:** unreachable knowledge source, blocked
  catalog, killed worker, too-short deadline, and a coordinator restarted
  *mid-task* — every one ending in a clean reap with zero survivors and a
  human-visible outcome; the restart case still delivered its report;
- **Hardening:** credential scans across all nine built images (clean),
  environment sweeps (no long-lived secrets anywhere), and the token
  revocation re-proof;
- **Open-item lockout:** every unresolved design question either locked or
  explicitly deferred to the next version, in writing.

The final stage's exit code — the number this book has been building toward
— was **zero**, with the acceptance task's report reading, in full:
`status: success`.

## 11.3 What the build demonstrated, measured

A few of the numbers the evidence pack holds:

| Property | Measured |
|---|---|
| Worker boot (role hydration → ready) | ~0.8–1.7 s (p50 785 ms across runs) |
| Memory read at boot | ~13 ms |
| Task credential lifetime cap | < 1 hour (assertion under 10 min) |
| Credential revocation → verified dead | ≤ 75 s |
| Registry-backed image pulls | no measurable boot overhead vs local |
| Completed runs ending with zero survivors | all of them |
| Roster edit → next worker uses it | no rebuild; next spawn |

## 11.4 The honest ledger of what was *not* smooth

This chapter has been the clean narrative; the real build was not clean.
Between the stages summarized above sit seventeen install runs of the final
stage alone, multiple replays of earlier gates against rebuilt images, and
roughly thirty-five distinct defects — each diagnosed, fixed, and ledgered.
Fourteen of those defects were in one stage (the durable core), nearly all
at component boundaries.

That is not an embarrassment to be edited out of the story; it is the
story. A build that never fails proves only that it never pushed far enough
to find its assumptions. The next chapter is therefore the one this book
was arguably written to contain: every significant failure, what it taught,
and the rule each one wrote into the design.
