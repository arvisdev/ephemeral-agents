# M5 Delivery Plan — End-to-End Single Flow + Hardening

| | |
|---|---|
| **Milestone** | M5 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | The full Version 1 flow, human-to-human: Software Engineer → Gateway → Manager → Topic → Temporal → ephemeral agents → hydrate → code to Project Repo → **destroy** → report back to the engineer. Plus the hardening that makes it defensible: zero orphans, scoped credentials, concurrency model locked. |
| **Depends on** | M0–M4 |
| **Closes** | R1, R2 (verified end-to-end), finalizes R3 + R4 |

## Work packages

**WP-5.1 — Wire the front of the house**
- Software Engineer (Hermes Desktop, outside the boundary) → Hermes Agent Gateway (VM-Xa) request path.
- **Manager Agent behavior (Note 1, made concrete):** look in the Available Agent List repo, search the agent index, find the full persona + skill and tools that can pull the markdown files, and create the appropriate agent artifact ("agent.f" as drawn); pass **repo location + repo folder + Behavioral Specification** into the task message (the M1 schema's `agent_ref` / `behavioral_spec` fields).
- Verify the two-way Manager ↔ RabbitMQ response path (owner-confirmed): correlation fields flow back to the manager.

**WP-5.2 — End-to-end run**
- One real request, all the way through: spawn → hydrate (all three sources, M4) → development work → **commit to the Project Repo on a task branch** → Hindsight write-on-end → **teardown** → result reported to the engineer.
- Evidence captured: workflow trace, container lifecycle log (spawn→reap), the actual commit, the report delivered.

**WP-5.3 — Failure-injection pass**
- Inject: agent crash mid-task, hydration source down (LLM-Wiki or Hindsight), repo unreachable, timeout. Each must end in: clean reap (zero orphans), a visible error to the human reporter with the reason, and a retryable or explicitly-terminal state (recorded).

**WP-5.4 — Credential hardening (R4 final)**
- Scan every image layer, container env, and workflow payload: **no long-lived owner credentials anywhere**.
- Per-run tokens: issued at spawn, scoped to one repo/branch, **revoked/expired at reap**; the M3b note from M3 now covers both agent types.
- Deliverable: a one-page credential model (who issues, TTL, scope, revocation) + the scan results.

**WP-5.5 — Concurrency model lock (R3 final)**
- Record the M3 decision (or make it now): one coder per task vs. N parallel — with the repo-write strategy (task branches / worktrees / serialization) implemented and evidenced.
- If N-parallel was chosen: re-run M3's concurrent-coders evidence now with the real images + M4 hydration.

**WP-5.6 — Report + docs**
- Human report format finalized (success/failure + artifact pointer, e.g. task-branch/PR link).
- Update `../ARCHITECTURE.md` + `../Initial-Foundation-Design-Document.md` open items with the POC's resolutions (VM naming decision D-8 already recorded at M0; message shapes, concurrency model, isolation level); list the genuinely-open items for **Version 2**.

## Deliverables

1. The full flow running M0→M5 as composed (one command from the engineer's side).
2. Evidence pack: E2E trace + lifecycle log + commit + report.
3. Failure-injection report (WP-5.3) with pass/fail per injection.
4. Credential-model page + negative scan results.
5. Decision-log entries closing R3, R4, message shapes (VM naming D-8 already resolved by owner 2026-10-25, recorded at M0); a "Version 2 open items" list.

## Acceptance criteria (done-when)

1. **E2E:** a real request completes end-to-end with **zero** surviving POC containers afterwards.
2. **Failure-injection:** all four injections end clean (reaped, reported, no leaks).
3. **Credentials:** the negative scan passes; no long-lived owner secret reachable from any container.
4. **Model locked:** concurrency + isolation + message shapes are recorded in the decision log (VM naming D-8 was already resolved by the owner); docs updated.
5. **Human loop closed:** the engineer sees the outcome (artifact pointer or error), matching the diagram's human-initiated/human-reported scope.

## Exit gate / POC complete

All five criteria ⇒ the Version 1 POC is **demonstrated feasible** and the feasibility verdict in `../Technical-Feasibility-Analysis.md` is confirmed by evidence. A failed criterion goes back to the owning milestone (R1→M2, R2→M1, R3/R4→M3) — do not paper over it at M5.

## Notes

- M5 adds **no new infrastructure**; it composes M0–M4 and hardens. If M5 discovers a missing piece, that's a finding about the earlier milestone's gate, not a new M5 work package.
- The Version 2 open items (from the foundation doc §7: LLM-Wiki curation ownership, Hindsight→LLM-Wiki promotion, long-term queue shape, multi-flow concurrency) are **recorded, not solved**, in this milestone.
