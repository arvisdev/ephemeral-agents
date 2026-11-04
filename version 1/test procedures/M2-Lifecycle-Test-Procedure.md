# M2 Test Procedure — Container Spawn & Reap System Gate (TP-M2)

**Gates progression to M3.** Before the M3 installation (`../installation-deployment/M3-Executors-Installation-and-Deployment.md`) begins, every **mandatory** case below must PASS. Evidence in `deploy/m2/evidence/`.

| | |
|---|---|
| **Milestone** | M2 (lifecycle manager — the POC's load-bearing custom piece) |
| **Companion docs** | `../delivery plans/M2-Container-Lifecycle-Manager-Delivery-Plan.md` · `../installation-deployment/M2-Lifecycle-Installation-and-Deployment.md` |
| **Entry criteria** | TP-M1 gate signed; `poc/lifecycle`, `poc/worker`, `poc/executor-stub` built + loaded on vm-xb; lifecycle + worker deployed (install section 2); no-pull deploy gate green |
| **Exit criteria** | All mandatory cases PASS + evidence filed → **M3 may begin** |

**Reference notation:** "Install sectionn" = section n of the companion installation-deployment document named in the header table above (e.g., "install section 3" in this doc = M2-Lifecycle-Installation-and-Deployment.md, section "3. Run the verification suite (the M2 gate tests)").

**Rules:**
- Run in order; a failure stops the procedure — remediate in M2, re-run from the failed case.
- "Zero survivors" after any case = `docker ps -a --filter label=poc.owner=lifecycle -q | wc -l` → 0.
- **Abort condition (from the M2 exit gate):** if TC-M2-04 (crash recovery) cannot be made to pass after remediation, **stop the POC sequence** — R1 is the load-bearing risk and M3 executors must not run on an unproven lifecycle.

## Mandatory test cases

| ID | Title | Procedure (install section 3) | Expected result | Pass criteria |
|---|---|---|---|---|
| TC-M2-01 | Clean path | `./tests/run-task.sh '{"task_id":"t001","ttl_s":30}'` | activity completes; container spawned with labels, then reaped | success; zero survivors; log shows spawn→complete→reap with t001 |
| TC-M2-02 | Failure path | `./tests/run-task.sh '{"task_id":"t002","inject":"exit-1"}'` | workflow surfaces failure (retry policy: 1 retry, then failed); container reaped | workflow = failed with reason; zero survivors |
| TC-M2-03 | Timeout path | `./tests/run-task.sh '{"task_id":"t003","inject":"hang","ttl_s":15}'` | reaper/timeout reaps at ~15 s; error surfaced | reaped within TTL + interval; zero survivors |
| TC-M2-04 | **Crash recovery** | start t004 (hang, ttl 120) → `docker kill worker` → wait 1 reaper interval → `docker start worker` | reaper (in `lifecycle`) reaps t004's container; workflow retries the activity; final state clean | **no orphan** at the `docker ps -a` check; zero survivors at end |
| TC-M2-05 | 10 mixed runs, zero survivors | `./tests/mixed-runs.sh 10` (5 success, 3 failure, 2 timeout) | all 10 lifecycle events logged; teardown idempotent | zero survivors; 10/10 events in logs |
| TC-M2-06 | No-pull rule | re-deploy via `deploy/m2/install.sh` with the pinned image **deleted** from the local store first, then restored; inspect the happy-path activity logs | deploy gate refuses to serve without the pinned image; no activity log ever contains an image pull | gate fires; zero pulls in logs |

## Informational (record, do not gate)

| ID | Title | What to record |
|---|---|---|
| I-M2-01 | Observability | per-task log file present in `/var/poc/tasks/<id>.log`; `/report` returns running tasks + last 10 reaps |
| I-M2-02 | Spawn latency | wall-time spawn→running for the stub (bounds later real-image expectations) |

## Evidence to file (`deploy/m2/evidence/`)

- `m2-tests.txt` — one block per TC: command, actual, PASS/FAIL, timestamp
- `m2-survivors-check.txt` — the zero-survivors count after TC-05
- `m2-task-logs/` — sample task logs (t001, t002, t003)
- results table signed → gate decision

## Gate decision

- **PASS:** TC-M2-01…06 → **M3 installation authorized**.
- **FAIL TC-M2-04:** remediate (reaper query filter, TTL labels, `chrony` clock), re-run TC-M2-04 then 05. If still failing after the M2 section 6 troubleshooting table → **abort gate**: re-baseline before M3 (owner decision).
- **FAIL TC-M2-06:** the no-pull gate is missing its enforcement — add it to the lifecycle entrypoint, re-run TC-M2-01 + 06.

## Regressions (re-run triggers)

- Any change to `services/lifecycle/`, its compose, or the stub image → full re-run.
- Docker version upgrade on vm-xb → re-run TC-M2-01…04.

## Sign-off (D-signoff, 2026-11-04)

The **Human-In-The-Loop (HITL)** — the engineer in the work loop — is the
sign-off authority for every gate in this POC, including this one. The
operator who ran the cases records the results table and evidence; the HITL
reviews the evidence and signs. (In a solo run the HITL and operator are the
same person — the signature asserts the evidence is complete, not that a
second person watched.)

| Field | Value |
|---|---|
| Signed by (HITL) | ______________________ |
| Date | ______________________ |
| Evidence pack | `deploy/m<N>/evidence/` (this TP's evidence list) |
