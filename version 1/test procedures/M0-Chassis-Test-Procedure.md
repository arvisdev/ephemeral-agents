# M0 Test Procedure — Infrastructure Foundation Gate (TP-M0)

**Gates progression to M1.** Before the M1 installation (`../installation-deployment/M1-Adapter-Installation-and-Deployment.md`) begins, every **mandatory** case below must PASS. Evidence is collected in `deploy/m0/evidence/`.

| | |
|---|---|
| **Milestone** | M0 (chassis) |
| **Companion docs** | `../delivery plans/M0-Chassis-Delivery-Plan.md` · `../installation-deployment/M0-Chassis-Installation-and-Deployment.md` |
| **Entry criteria** | PVE shell access, IP pool, `deploy/m0/` committed to the POC repo |
| **Exit criteria** | All mandatory cases PASS + evidence filed + decision log written → **M1 may begin** |

**Reference notation:** "Install §n" = section n of the companion installation-deployment document named in the header table above (e.g., "install §1" in this doc = M0-Chassis-Installation-and-Deployment.md, section "1. Provision the VMs").

**Rules:**
- Run cases in order; a failure stops the procedure — remediate within M0 (the M0 install doc §8), then re-run the failed case **and everything after it**.
- Record PASS/FAIL + timestamp + operator in `deploy/m0/evidence/tp-m0-results.md` (one line per case; attach outputs for §4–§7 evidence items).
- Remediation that changes any compose file or script → re-run the full procedure (the chassis must be proven from the repo, not from edits-in-place).

## Mandatory test cases

| ID | Title | Procedure (commands in the install doc) | Expected result | Pass criteria |
|---|---|---|---|---|
| TC-M0-01 | VMs provisioned & configured | Install §1–§2: 3 VMs boot (D-wiki-git: vm-llm-wiki dropped); hostname, static IP, chrony, `poc` ssh user | `ssh poc@$VM_XA / $VM_XB / $VM_3` from build host (hostnames from `deploy/m0/hosts.env`: vm-xa, vm-xb, vm-hindsight per the D-8 mapping); `timedatectl` shows NTP synced on all | all 3 reachable, NTP synced |
| TC-M0-02 | Docker on all VMs | `docker run --rm alpine echo ok` + `docker compose version` on each VM | `ok` ×3; compose v2 | zero failures |
| TC-M0-03 | RabbitMQ service + topology | Install §3: compose healthy; mgmt-API checks for exchange `agent.tasks`, queue `agent.tasks.durable`, binding, DLQ queue | all 4 objects present; healthcheck `healthy` | objects exist; healthcheck green |
| TC-M0-04 | Cross-VM publish/consume | From **vm-xa**, publish 1 message to `agent.tasks` → check queue depth on vm-xb | queue shows 1 message | 1 |
| TC-M0-05 | Temporal service | Install §4: compose healthy; `temporal operator cluster health`; `temporal operator namespace list` | members HEALTHY; `default` listed | both true |
| TC-M0-06 | **Go SDK smoke (language gate)** | Install §4b: `go run . -addr vm-xb:7233` in `deploy/m0/go-sdk-smoke/` | prints `hello from greet` — a **Go** workflow completed | must PASS for the language decision to hold |
| TC-M0-07 | Network matrix | Install §5: ssh-22 pairwise all 3 VMs (D-wiki-git); `curl -sI https://github.com` from vm-xa/vm-xb; `nc` 7233/5672 on vm-xb | matrix 100% | zero failed probes (recorded) |
| TC-M0-08 | Idempotent re-deploy | Re-run `deploy/m0/install.sh` against the live chassis | all services still healthy; no data loss (RabbitMQ queue + Temporal PG intact) | exit 0; healthchecks green |

## Evidence to file (`deploy/m0/evidence/`)

- `tp-m0-results.md` — the result table (case, PASS/FAIL, timestamp, operator)
- `network-matrix.txt` — raw matrix output (install §5)
- `go-sdk-smoke.log` — the TC-M0-06 run output
- `decision-log.md` entry — VM naming scheme **(a)** + hostname mapping **(b)** + **Go language decision (c)** (install §6)

## Gate decision

- **PASS:** all TC-M0-01…08 PASS → sign the results table → **M1 installation authorized**.
- **FAIL:** any mandatory case fails → remediate in M0, re-run from the failed case. No M1 work under a red gate.
- TC-M0-06 failing means the Go toolchain/SDK path is broken — it is a language-gate failure, not a chassis defect; fix the smoke harness before anything else.

## Regressions (re-run triggers)

- Any change to `deploy/m0/*` (compose, scripts, env) → full re-run.
- Proxmox host maintenance (kernel upgrade, LVM changes) → re-run TC-M0-01/02/07 at minimum.

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
