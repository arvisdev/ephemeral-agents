# NFR-1 Gap-Closure Plan — "junior developer completes full installation and testing without any other help"
Date: 2026-10-30 · Requirement: BRD NFR-1 (verbatim, commit b1c91e4) · Baseline: junior-dev readiness analysis of 2026-10-30, repo @ b1c91e4

## The requirement, operationalized
A junior developer, given only this repo and the owner-supplied inputs, must be able to:
1. create the four VMs and the PVE chassis (M0),
2. deploy every component and container (M0–M5),
3. run and pass every gate test (TP-M0…TP-M5),
without asking anyone anything the repo doesn't answer.

## Gap inventory (complete, verified against the tree)

**G1 — The Go services do not exist** (no `.go`, no `go.mod` anywhere; zero code blocks in any doc). Required by script asset checks:
| Milestone | Missing source |
|---|---|
| M1 | `services/topic-bridge/cmd/adapter/main.go` |
| M2 | `services/lifecycle/cmd/lifecycle/main.go`, `services/lifecycle/cmd/worker/main.go` |
| M4 | `services/hindsight-api/`, `services/llmwiki-api/` |
| M5 | `services/gateway/`, `services/manager/` |
Behavior is specified (install docs + delivery plans); implementation is not. This is the only gap that requires developer skill, and it blocks M1–M5 entirely.

**G2 — All non-M0 deliverable files are missing** (docs carry contents; files don't):
M0: `deploy/m0/rabbitmq/compose.yaml`, `deploy/m0/temporal/compose.yaml`, `deploy/m0/go-sdk-smoke/` (Go program), `deploy/m0/.env.example` · M1: `deploy/m1/{Dockerfile,compose.yaml}`, `deploy/m1/tests/{publish.sh,assert.sh}` · M2: `deploy/m2/{Dockerfile.services,compose.yaml}`, `deploy/m2/executor-stub/{Dockerfile,entrypoint.sh}`, `deploy/m2/tests/{run-task.sh,mixed-runs.sh}` · M3: `deploy/m3/hermes-ephemeral/{Dockerfile,entrypoint.sh}`, `deploy/m3/claude-coder/{Dockerfile,entrypoint.sh}`, `deploy/m3/issue-token.sh`, `deploy/m3/tests/boot-time.sh` · M4: `deploy/m4/hindsight/{compose.yaml,initdb.sql,Dockerfile.api}`, `deploy/m4/llmwiki/{compose.yaml,Dockerfile.api}` · M5: `deploy/m5/compose.yaml`, `deploy/m5/e2e-deploy.sh`, `deploy/m5/report-contract.md`.
Only M0's test harness (`declare-topology-test.sh`, `rmq-stub.py`) is committed. Test assets count double under this requirement: "testing without help" needs them too.

**G3 — VM creation has a boot-media hole.** `qm create --virtio0` makes empty disks; the only OS-install guidance is "ISO/cloud-init per the estate's existing pattern" — estate knowledge the junior doesn't have. A literal follower gets four unbootable VMs.

**G4 — No worked network example.** Netplan is "10.0.90.x style from the PVE LAN pool"; `hosts.env.example` has no example IP set; netplan/hosts.env/DNS must agree silently across 4 VMs and every later script.

**G5 — Owner-supplied inputs are scattered.** GitHub App + key (M3), Available Agent List repo + read token, `CONTENT_REPO` + `LLMWIKI_READ_TOKEN` (M4), `EMBEDDING_PROVIDER` + key, TTL/cap config values — each mentioned only inside its own milestone doc; nothing tells the junior to collect them before starting.

**G6 — No single readiness check.** The workstation checklist (tools, SSH, sudo, hosts) lives as prose in `deploy/README.md`; nothing validates it before M0.

**G7 — Minor doc aids.** `deploy/m0/decision-log.md` is referenced but has no template; the M0 gate check "decision-log entry exists" leaves the junior to invent the format.

## The plan (4 phases, dependency-ordered)

**Phase 1 — Doc patches (junior-doable, ~1 day, no new knowledge needed)**
1. M0 §1: add the boot step — attach the estate's Ubuntu cloud-init template (`qm set <vmid> --ide2 local:iso/<template>.iso` or `qm importdisk` + `ciuser=poc` + SSH key via cloud-init), matching the existing `poc` user/chrony/netplan steps. Optionally add a `provision` sub-step to `install.sh`'s `provision_vms`.
2. `hosts.env.example`: add a commented worked example (4 concrete IPs from a sample pool) and the rule "same IPs in netplan, hosts.env, and DNS/hosts".
3. `deploy/README.md`: add an "Owner must hand you these" checklist (G5's six items) before the first-time setup section.
4. Add `deploy/check-workstation.sh` (G6): verifies docker/buildx/go/temporal CLI/gh/rsync, SSH key auth as `poc` to PVE + 4 VMs, passwordless sudo, and `--check-only` per-milestone pass.
5. Ship a `decision-log.md` template in `deploy/m0/` (G7).
Exit: every doc gap that is fixable by editing text is closed; `bash -n` + M0 harness re-run.

**Phase 2 — Transcribe the authorable assets (junior-doable, 2–3 days, mechanical)**
Create every G2 file verbatim from its install doc (compose files, Dockerfiles, entrypoints, `initdb.sql`, `issue-token.sh` per its ~20-line spec, test scripts per their doc blocks, `go-sdk-smoke` per M0 §5, `.env.example`). Copy `schemas/message-v1.json` into `services/topic-bridge/schemas/`. Verify after each milestone: `bash deploy/m<n>/install.sh --check-only` must report only the Go directories missing.
Exit: `--check-only` for all six milestones fails on *nothing but* `services/*` paths (and M0 assets fully green).

**Phase 3 — Build the Go services (the real work; developer skill; order = dependency)**
1. **M1 topic-bridge first** — it unblocks M2–M5. Implement the five-step correctness contract exactly as M1 doc §1 pins it (prefetch=1, manual ack, schema-validate → DLQ+ack, `StartWorkflow{WorkflowID: task_id}`, ack-after-start incl. `ErrWorkflowExecutionAlreadyExists`). `go.mod` with `amqp091-go` + Temporal Go SDK; unit tests; pass T1–T5 locally against the M0 stub, then TP-M1's six cases.
2. **M2 lifecycle + worker** — HTTP API + label reaper + Temporal worker + response publisher (default exchange). V4 crash-recovery is load-bearing: if it can't pass, STOP per the script's abort condition.
3. **M4 hindsight-api + llmwiki-api** — pgvector R/W API, same-embedding-model ingest/query, client package for executor entrypoints.
4. **M5 gateway + manager** — `/healthz`, agent-index resolve, `schemas/message-v1.json` publish with manager-only `ttl_s = max(3 × B1 p50, estimate)`, response consume filtered by `task_id` → `/reports/<task_id>.json`.
Exit per service: `go vet` + unit tests + the milestone's script gate tests pass; TP signed.

**Phase 4 — The NFR-1 proof run**
Execute M0→M5 in sequence as the gates demand, each TP signed before the next script. Then verify the requirement itself: a junior developer who did **not** build the assets performs the full run (VMs → containers → all gates) using only the repo; every question they had to ask becomes a doc patch (that Q&A loop is the only honest test of "without any other help"). When TP-M5 is signed on that run, NFR-1 is met by evidence.

## Effort & risk summary
- Phases 1–2: ~3–4 junior-days; no risk; immediately shrinks the failure surface.
- Phase 3: the project's real remaining work (~1–2 dev-weeks for one Go developer, M1 first). Highest risk: M2-V4 (crash recovery) and M4 retrieval quality (TC-M4-03).
- Do not start Phase 4 until Phases 1–3 exit criteria hold — `--check-only` green for all six milestones is the preflight.
