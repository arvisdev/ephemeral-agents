# M5 Test Procedure — End-to-End + Hardening Gate (TP-M5)

**The final gate: POC complete.** No milestone follows M5 — all cases below must PASS to convert the feasibility verdict from analysis to **evidence**. Evidence in `deploy/m5/evidence/`.

| | |
|---|---|
| **Milestone** | M5 (front-of-house, E2E, hardening, open-item lockout) |
| **Companion docs** | `../delivery plans/M5-End-to-End-Hardening-Delivery-Plan.md` · `../installation-deployment/M5-End-to-End-Installation-and-Deployment.md` |
| **Entry criteria** | TP-M0…TP-M4 all signed; `deploy/m5/e2e-deploy.sh` completes S1–S7 green (S7 = stub smoke) |
| **Exit criteria** | All mandatory cases PASS + evidence pack filed + open items locked → **POC demonstrated feasible** |

**Reference notation:** "Install §n" = section n of the companion installation-deployment document named in the header table above (e.g., "install §4" in this doc = M5-End-to-End-Installation-and-Deployment.md, section "4. End-to-end acceptance run (the POC gate)").

**Rules:**
- Run in order; a failure stops the procedure — remediate in the **owning milestone** (named in each case), re-run that gate's owning cases, then resume M5 from the failed case. M5 adds no new Temporal code — every Temporal component remains the Go M1/M2 services.
- Every case that spawns containers ends with the zero-survivors check.

## Mandatory test cases

| ID | Title | Procedure (install §) | Expected result | Pass criteria |
|---|---|---|---|---|
| TC-M5-01 | Front-of-house | §3 S6 + §1: gateway `/healthz` up on vm-xa; manager consuming; engineer desktop sends `{title, agent_capability, task_brief}` | request lands as exactly 1 new `TaskWorkflow` (Temporal CLI) | 1 workflow, correlation_id visible in the message |
| TC-M5-02 | **E2E acceptance run** | §4 E1: one real request, human to human | the full artifact chain exists | (a) temporal trace shows the run; (b) container labels seen alive, **zero survivors** after; (c) commit on `origin/task/<id>`; (d) `GET /gateway/reports/<task_id>` returns the report JSON (branch + commit + status) |
| TC-M5-03 | Failure injections F1–F4 | §5 runbook: pause LLM-Wiki / block catalog 443 / `pkill -9` agent / TTL too short — one task each | each: clean reap (zero survivors) + **human-visible error** in the report with the reason | 4/4 clean + reported; policy (error vs. degrade) recorded |
| TC-M5-04 | Credential hardening | §6: trivy secret scan across all 10 deployed images (the full §6a loop set); `docker inspect` env sweep on every running container; token lifecycle re-check (M3 TC-M3-05) | no long-lived owner secret anywhere; issued tokens dead post-reap | all three clean |
| TC-M5-05 | Concurrency model locked | §7: the M3-chosen model's evidence re-validated on the **real** images (sequential task branches, or N-parallel worktrees) + decision-log entry | the recorded model is what actually runs | decision log + matching evidence |
| TC-M5-06 | Human report loop | after TC-M5-02: the engineer's desktop receives the outcome without asking Temporal/RabbitMQ directly | report delivered via the gateway (correlation path, the two-way Manager ↔ RabbitMQ variant) | engineer sees artifact pointer or error; path logged |
| TC-M5-07 | Docs closed | §7 checklist: decision log complete (naming D-8 recorded, message shapes, concurrency, isolation, curation owner); `../ARCHITECTURE.md` + foundation doc updated; **Version 2 open items list** written | all five open items either locked or explicitly deferred | checklist 100%; docs committed |

## Informational (record, do not gate)

| ID | Title | What to record |
|---|---|---|
| I-M5-01 | Full evidence pack | temporal trace export, lifecycle log, commit SHA, report payload → `deploy/m5/evidence/` (the POC's closing artifact) |
| I-M5-02 | Timing profile | wall-times per stage of one E2E run (request → report) |

## Evidence to file (`deploy/m5/evidence/`)

- `e2e-run/` — TC-M5-02 artifact chain (trace, labels, commit, report JSON)
- `failure-injections.txt` — TC-M5-03, one block per F1–F4
- `hardening-scan.txt` — TC-M5-04 trivy + inspect outputs
- `decision-log.md` — TC-M5-05/07 entries
- results table signed → **POC completion decision**

## Gate decision (POC complete)

- **PASS:** TC-M5-01…06 all PASS, TC-M5-07 complete → the verdict in `../Technical-Feasibility-Analysis.md` ("technically feasible") is **confirmed by evidence**. Archive the evidence pack; the POC baseline is closed.
- **FAIL any TC-M5-01…06** → remediate in the owning milestone (R1→M2, R2→M1, R3/R4→M3, R6/R7→M4, front-of-house→M5), re-run that milestone's gate cases, then resume M5 from the failed case. **Do not paper over at M5.**
- **TC-M5-07 incomplete:** the POC can run but the versioning record is broken — finish it before declaring complete (the Version 2 open-items list is what makes the next iteration start cleanly).

## Regressions (re-run triggers)

- Any image rebuild or service redeploy → re-run TC-M5-02 (+ 03 if a failure path touched).
- Credential mechanism change → TC-M5-04.
- Concurrency model change → TC-M5-05 + M3 TC-M3-07.
