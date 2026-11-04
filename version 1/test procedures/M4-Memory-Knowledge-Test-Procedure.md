# M4 Test Procedure — Memory + Knowledge Gate (TP-M4)

**Gates progression to M5.** Before the M5 installation (`../installation-deployment/M5-End-to-End-Installation-and-Deployment.md`) begins, every **mandatory** case below must PASS. Evidence in `deploy/m4/evidence/`.

| | |
|---|---|
| **Milestone** | M4 (Hindsight R/W API + llm-wiki git-repo hydration, wired into the lifecycle) |
| **Companion docs** | `../delivery plans/M4-Memory-and-Knowledge-Delivery-Plan.md` · `../installation-deployment/M4-Memory-Knowledge-Installation-and-Deployment.md` |
| **Entry criteria** | TP-M3 gate signed; Hindsight service deployed (install §1); llm-wiki repo clone settings wired into the M2 config (install §2–3); executor env updated (install §3) |
| **Exit criteria** | All mandatory cases PASS + evidence filed → **M5 may begin** |

**Reference notation:** "Install §n" = section n of the companion installation-deployment document named in the header table above (e.g., "install §4" in this doc = M4-Memory-Knowledge-Installation-and-Deployment.md, section "4. Verification (the M4 gate tests)").

**Rules:**
- Run in order; a failure stops the procedure — remediate in M4, re-run from the failed case.
- Every case that spawns containers ends with a zero-survivors check (`docker ps -a --filter label=poc.owner=lifecycle -q | wc -l` → 0) — M2's property must hold with the new sources attached.
- Language decision: no new Temporal code — the M2 Go worker still drives everything; confirm in TC-M4-04 that the existing worker (unmodified) handles the new env.

## Mandatory test cases

| ID | Title | Procedure (install §4) | Expected result | Pass criteria |
|---|---|---|---|---|
| TC-M4-01 | Hindsight network R/W | from inside a spawned agent container: `curl "$HINDSIGHT_URL/memory/read?task_id=t1&scope=cross-task"` then `curl -XPOST .../memory/write -d '{...}'` → re-read | write persists; read returns it | read-back equals the written body (JSON compare) |
| TC-M4-02 | Cross-task read-back | task t1 (scope `cross-task`) writes `{learning:"use X for Y"}` → reaped; task t2 hydrate runs | t2's hydrate payload contains t1's learning | present in t2's task payload (recorded) |
| TC-M4-03 | llm-wiki clone + read ×3 | from a spawned agent container (D-wiki-git — replaces the former wiki-query check): `git clone --depth 1 "$LLM_WIKI_REPO_URL"` then look up: (a) a design standard, (b) a dev best practice, (c) a skill/tool topic | the clone contains the right doc for each; the relevant file/section is findable via the repo's README/directory layout | 3/3 found (record clone SHA + file path) |
| TC-M4-04 | Catalog regression + wiring | re-run M3's TC-M3-02 self-report check **with both M4 sources live**; confirm the M2 worker (unchanged Go binary) picked up the new env | self-report still correct; sources include llm-wiki (clone) + hindsight; no M2 code redeploy needed | self-report matches; worker image tag unchanged |
| TC-M4-05 | Hydration latency (3 runs) | time hydrate fan-out: llm-wiki shallow clone, Hindsight read, catalog fetch (per source + total) | total < M2 `TTL_S` headroom (margin ≥ 50%) | 3/3 within bound; actuals recorded |
| TC-M4-06 | Curation cycle | push a new standard doc to the **llm-wiki repo** → re-clone (fresh shallow clone) → look it up | new doc present and readable in the fresh clone; old docs still readable | new doc found; no regression on TC-M4-03's lookups |
| TC-M4-07 | Data isolation | inspect vm-hindsight (and vm-xb) compose + running containers | Hindsight uses its **own** Postgres instance, separate from Temporal's (M0 rule: never share DBs); **no wiki VM/service exists** (D-wiki-git) | distinct PG containers, distinct volumes; zero llmwiki containers/images present |

## Informational (record, do not gate)

| ID | Title | What to record |
|---|---|---|
| I-M4-01 | Curation evidence | the curation note in the llm-wiki repo's `CHANGELOG.md` — owner = the Human-In-The-Loop (HITL), append-only need-driven cadence (D-curation, 2026-11-04; tracked there per D-wiki-git, not in the decision log) |
| I-M4-02 | Embedding provider cost/rate | provider chosen, model, observed rate-limit headroom at POC query volume |

## Evidence to file (`deploy/m4/evidence/`)

- `readback.txt` — TC-M4-01/02 raw JSON
- `retrieval-hits.md` — TC-M4-03/06 clone lookups + file paths found
- `latency-table.txt` — TC-M4-05 (3 runs × sources + total)
- `isolation-check.txt` — TC-M4-07 container/volume list
- results table signed → gate decision

## Gate decision

- **PASS:** TC-M4-01…07 all PASS (07 is the store-isolation check — a shared Postgres FAILS the gate) → **M5 installation authorized**.
- **FAIL TC-M4-02** (read-back empty) → scope-rule mismatch: writes used `session` where `cross-task` was intended (install §3.4) — fix the executor write, re-run 01 + 02 + 04.
- **FAIL TC-M4-03** → clone/read failure: check `LLM_WIKI_REPO_URL` + read token + egress to github.com (install §6 first row); re-run 03 + 06.
- **FAIL TC-M4-05** → hydration too slow for TTL: check clone latency / provider latency; resize TTL only via the decision log.

## Regressions (re-run triggers)

- A push to the llm-wiki repo + fresh-clone verification → re-run TC-M4-06 only.
- Hindsight schema change or provider change → full re-run.
- M2 worker/lifecycle redeploy → re-run TC-M4-04.

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
