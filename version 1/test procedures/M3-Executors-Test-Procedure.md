# M3 Test Procedure — Executor Spikes Gate (TP-M3)

**Gates progression to M4.** Before the M4 installation (`../installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md`) begins, every **mandatory** case below must PASS. Evidence in `deploy/m3/evidence/`.

| | |
|---|---|
| **Milestone** | M3 (the two To-verify components — **point of last cheap exit** for the whole POC) |
| **Companion docs** | `../delivery plans/M3-Executor-Spikes-Delivery-Plan.md` · `../installation-deployment/M3-Executors-Installation-and-Deployment.md` |
| **Entry criteria** | TP-M2 gate signed; Available Agent List + Project repos exist; GitHub App installed; `poc/hermes-ephemeral:1.0.0` + `poc/claude-coder:1.0.0` built, loaded, and selected in M2 config |
| **Exit criteria** | All mandatory cases PASS + evidence filed + concurrency decision logged → **M4 may begin** |

**Reference notation:** "Install §n" = section n of the companion installation-deployment document named in the header table above (e.g., "install §4" in this doc = M3-Executors-Installation-and-Deployment.md, section "4. Verification (the M3 gate tests)").

**Rules:**
- Run in order; a failure stops the procedure — remediate in M3, re-run from the failed case.
- **Abort condition (from M3):** if TC-M3-01/02 (Hermes boot-time persona load) fail, try the documented sidecar fallback (hydration sidecar prepares agent config before the agent process starts); if **that** also fails → **stop the POC sequence** and re-baseline the executor design (the R5 abort condition). A failed gate here is the last point where an executor swap is cheap.
- Language decision: no new Temporal code in M3 — verify M2 services still run unchanged (TC-M3-04 re-uses them).

## Mandatory test cases

| ID | Title | Procedure (install §4) | Expected result | Pass criteria |
|---|---|---|---|---|
| TC-M3-01 | Hermes boot-time ×3 | `./tests/boot-time.sh hermes 3` (M2 spawn; measure container start → `self-report.json` ready) | 3 successful boots; times recorded | 3/3; p50 recorded **and reproducible** (Δ within 50% on re-run) |
| TC-M3-02 | Persona correctness | spawn with `AGENT_REF` → `agents/dev-agent`; inspect `self-report.json` | role, skills[], spec match the catalog contents for that folder; sources fields present | exact match to `available-agent-list/agents/dev-agent/*` |
| TC-M3-03 | Failure paths | (a) `AGENT_REPO_URL` unreachable → run; (b) malformed persona (bad index entry) → run | both: `self-report.json` with `error`, exit 2, container reaped, M2 surfaces the error | 2/2 clean, zero survivors |
| TC-M3-04 | Coder real work | M3b task: "add a /healthz handler" on the POC project | Claude Code makes the change; commit + push to `task/<id>`; `/work/report.json` `commit_sha` matches `git log` | commit exists on origin with the changed files |
| TC-M3-05 | Token lifecycle | after reap, query GitHub with the issued installation token | 401 (revoked/expired per M3 §1c) | unusable post-reap; token TTL ≤ 1 h |
| TC-M3-06 | **Secret hygiene** | install §5: `env` dump of both images; `docker history` grep; `trivy` secrets scan | no baked-in secrets; trivy secrets = 0 across both images | all three clean |
| TC-M3-07 | Concurrency model evidence | default: 3 sequential single-coder tasks (3 branches pushed, no conflict). If N-parallel chosen: 2 concurrent coders via worktrees | all branches land; no lost work | decision **recorded** in the decision log + the matching evidence run attached |

## Informational (record, do not gate)

| ID | Title | What to record |
|---|---|---|
| I-M3-01 | Toolchain notes | pinned image tags + Claude Code CLI version used (reproducibility record) |
| I-M3-02 | Coder wall-time | task wall-time vs. `TTL_S` headroom (feeds the M5 TTL sizing) |

## Evidence to file (`deploy/m3/evidence/`)

- `boot-time-table.txt` — TC-M3-01 (3 runs + reproducibility re-run)
- `self-reports/` — TC-M3-02/03 raw `self-report.json` files
- `m3b-commit.txt` — TC-M3-04 commit SHA + `git log --oneline` of the task branch
- `token-check.txt` — TC-M3-05 curl output
- `secret-scan.txt` — TC-M3-06 outputs
- `concurrency-decision.md` — TC-M3-07 decision + evidence reference
- results table signed → gate decision

## Gate decision

- **PASS:** TC-M3-01…07 (07 = decision logged + matching evidence) → **M4 installation authorized**.
- **FAIL TC-M3-01/02** → sidecar fallback → still failing → **abort gate, re-baseline** (owner decision).
- **FAIL TC-M3-04** → repo/branch/token plumbing (M3 §1c, §2b); re-run 04 + 05.
- **FAIL TC-M3-06** → secrets in an image layer: find the `RUN` layer that wrote them, rebuild without them; re-run 06 (never ship an image that fails this).

## Regressions (re-run triggers)

- Any executor image rebuild or catalog repo layout change → full re-run.
- GitHub App permission changes / token mechanism change → TC-M3-04, 05, 06.

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
