# M2 test work packages — behavioral specs for V1–V5

Canonical test inputs for the M2 verification suite (V1–V5). Written in the
FS-Eng worker's own vocabulary (work package / acceptance criteria /
artifact-first, per the Agents org's ephemeral worker role). The stub executor
(`poc/executor-stub`) is the contract-bound stand-in for a real executor: it
echoes the config envelope (task_id, agent_ref, behavioral_spec) to stdout,
writes `/work/marker`, and sleeps for the TTL — so each package below maps
onto exactly one observed stub behavior.

These packages are the `behavioral_spec` field of every test payload in
`deploy/m2/install.sh` (V1–V4) and `deploy/m2/tests/mixed-runs.sh` (V5).
Re-running M0–M5 on a fresh clone reproduces this suite as-is.

## WP-T01 — clean path (V1; mixed "success" runs)

Boot the ephemeral executor, write the start marker to `/work/marker`, and
echo the config envelope (task_id, agent_ref, behavioral_spec) to stdout as
the artifact. **Acceptance criteria:** marker exists; stdout contains all
three config fields; process exits 0 within the TTL.
Workflow expectation: `success`; no `poc.task=t001` container survives.

## WP-T02 — failure path (V2; mixed "failure" runs)

Same envelope as WP-T01, but the package is deliberately malformed — the
executor must fail loudly. **Acceptance criteria:** non-zero exit; failure
reason written to stdout before exit.
Workflow expectation: `failed`; container reaped.

## WP-T03 — timeout path (V3; mixed "timeout" runs)

Hold the executor alive for the full TTL with no artifact returned.
**Acceptance criteria:** the lifecycle TTL backstop (`poc.expires` label)
fires first; the reaper reaps the container; the workflow surfaces
`timed-out`.
Workflow expectation: `timeout`, reaped.

## WP-T04 — crash path (V4 — the load-bearing test)

Identical bounded-runtime behavior to WP-T03, with the added condition that
the worker process dies mid-package. **Acceptance criteria:** the orphaned
container is still reaped by the independent lifecycle reaper — zero
survivors, regardless of worker availability. If this cannot pass, the POC
abort condition fires (M2 install doc §3).

## V5 mixed mapping

`tests/mixed-runs.sh 10` cycles: 5 × WP-T01 (success), 3 × WP-T02
(failure), 2 × WP-T03 (timeout), then counts survivors — expect 0.

## Payload field values (all cases)

| Field | Value | Note |
|---|---|---|
| `agent_ref` | `agent-roster/ephemeral-org/eph-fullstack-engineer` | real roster folder (repo location + folder holding the 5 markdowns) |
| `repo_target` | `repos/ephemeral-poc-demo` | seeded demo repo at `deploy/m2/tests/poc-demo-repo/` |
| `requester` | `principal` | carries the response path; correlated on task_id |
| `ttl_s` | per-case (30 / 30 / 15 / 120) | doc §3 prescribes; V3–V5 timeouts exercise the backstop |
| `task_id` | unique per run | idempotency key = Temporal workflow ID |
