# M1 Test Procedure — RabbitMQ → Temporal Adapter Gate (TP-M1)

**Gates progression to M2.** Before the M2 installation (`../installation-deployment/M2-Lifecycle-Installation-and-Deployment.md`) begins, every **mandatory** case below must PASS. Evidence in `deploy/m1/evidence/`.

| | |
|---|---|
| **Milestone** | M1 (adapter) |
| **Companion docs** | `../delivery plans/M1-RabbitMQ-Temporal-Adapter-Delivery-Plan.md` · `../installation-deployment/M1-Adapter-Installation-and-Deployment.md` |
| **Entry criteria** | TP-M0 gate signed; `poc/topic-bridge:1.0.0` built, loaded on vm-xb, deployed via install §3; consumer connected and consuming |
| **Exit criteria** | All mandatory cases PASS + evidence filed → **M2 may begin** |

**Reference notation:** "Install §n" = section n of the companion installation-deployment document named in the header table above (e.g., "install §4" in this doc = M1-Adapter-Installation-and-Deployment.md, section "4. Verification run (the M1 gate tests)").

**Rules:**
- Run in order; a failure stops the procedure — remediate in M1, re-run the failed case + everything after.
- Between every scenario that publishes workflows, record the baseline count (`temporal workflow list --type TaskWorkflow | grep -c running`) so increments are attributable.
- All adapter code under test must be **Go** (language decision, install §1 header) — a non-Go implementation fails the gate regardless of behavior.

## Mandatory test cases

| ID | Title | Procedure (install §4 harness) | Expected result | Pass criteria |
|---|---|---|---|---|
| TC-M1-01 | Poison-pill → DLQ | `./tests/publish.sh --invalid 3` | 3 messages land in `agent.tasks.dlq` with a reason property; consumer process still up; main queue depth 0 | DLQ count = 3; consumer healthy |
| TC-M1-02 | N → N (N=50) | baseline count B; `./tests/publish.sh 50`; wait 5 s | running workflows = B + 50 | exact +50 |
| TC-M1-03 | Duplicate idempotency | `./tests/publish.sh 50 --dup` (same 50 `task_id`s) | running workflows unchanged (no second start) | count = B + 50 still; adapter log shows "already exists → ack" |
| TC-M1-04 | Kill -9, no loss | `./tests/publish.sh 20 &` → after 1 s `docker kill topic-bridge` → `docker start topic-bridge` → wait | +20 workflows; **no** duplicate starts from redelivered unacked messages | count = B + 70; no error spike |
| TC-M1-05 | Temporal restart recovery | `docker restart temporal`; wait 30 s; `./tests/publish.sh 5` | consumer recovers automatically (restart policy); 5 new workflows start | +5; no manual intervention |
| TC-M1-06 | Rollback / re-deploy | `docker compose down topic-bridge` → `up -d` | consumer resumes from the durable queue; un-consumed messages not lost | queue drains to the new count |

## Informational (record, do not gate)

| ID | Title | What to record |
|---|---|---|
| I-M1-01 | Throughput sanity | wall-time for 50-message burst + `docker stats` snapshot — establishes the POC's latency baseline |
| I-M1-02 | Log hygiene | confirm json-file rotation caps work (no unbounded log growth after burst) |

## Evidence to file (`deploy/m1/evidence/`)

- `m1-tests.txt` — one block per TC: baseline, command, actual count, PASS/FAIL, timestamp
- `m1-dlq-sample.json` — one DLQ message showing the reason property
- results table signed → gate decision

## Gate decision

- **PASS:** TC-M1-01…06 all PASS (and under test the code is Go) → **M2 installation authorized**.
- **FAIL TC-M1-03** (duplicates start) → the `WorkflowID=task_id` contract is broken — this is the adapter's core correctness property; fix and re-run TC-M1-02…05 (not just 03).
- **FAIL TC-M1-04** (loss on kill) → ack-after-start ordering violated; fix and re-run TC-M1-04 + 05.
- No M2 work under a red gate.

## Regressions (re-run triggers)

- Any change to `services/topic-bridge/` source, `deploy/m1/compose.yaml`, or the message schema → full re-run.
- RabbitMQ/Temporal image upgrades → re-run TC-M1-01, 05, 06.

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
