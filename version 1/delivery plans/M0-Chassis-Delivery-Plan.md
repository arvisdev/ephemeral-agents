# M0 Delivery Plan — Chassis

| | |
|---|---|
| **Milestone** | M0 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | Proxmox 9.x + VMs + Docker + RabbitMQ + Temporal standing up and reachable; VM numbering locked. |
| **Depends on** | Nothing (first milestone) |
| **Retires risk** | R8 (service discovery), R9 (VM numbering) |

## Work packages

**WP-0.1 — Lock VM numbering and naming (decision D-numbers)**
- Finalize the VM naming for this POC: VM-Xa, VM-Xb, VM-2 (LLM-Wiki), VM-3 (Hindsight) — or a new scheme; record the decision and update `../ARCHITECTURE.md` open items.
- Assign hostnames + network role per VM.
- Deliverable: decision-log entry (2–4 lines) + updated docs; record the **language decision: Go for all Temporal code** (owner instruction: speed + size).

**WP-0.2 — Provision VMs on Proxmox 9.x**
- 4 KVM VMs: VM-Xa, VM-Xb, VM-2, VM-3 (Debian/Ubuntu, matching the running Proxmox estate).
- Sizing: starter VMs (2 vCPU / 4 GB is enough for POC scale); VM-Xb slightly larger (hosts queue + orchestrator + containers).
- Network: a flat/bridged L2 or routed segment where all four VMs reach each other, plus outbound to GitHub. Record the chosen topology.

**WP-0.3 — Docker on every VM**
- Install Docker; verify `pull` + `run` of a hello-container on each VM; pin base image versions.
- Deliverable: per-VM compose files (or install script) for Docker.

**WP-0.4 — RabbitMQ (Topic) on VM-Xb**
- RabbitMQ container with a **topic exchange** + durable queues; health endpoint (management plugin) reachable from VM-Xa.
- Define the v1 topic name + routing pattern (e.g. `agent.tasks.#`) — this shapes M1's message schema.
- Verify cross-VM publish (VM-Xa) → consume (VM-Xb).

**WP-0.5 — Temporal on VM-Xb**
- Temporal Server container with a **PostgreSQL** backend (Docker Compose, matching the proven PG pattern from prior deployments).
- Verify: `temporal workflow start` + a trivial greet workflow/worker runs to completion (CLI + one SDK sample).
- Note: VM-3 (Hindsight, M4) also runs Postgres — a **separate** instance per VM; do not share.

**WP-0.6 — Network verification matrix**
- Test and record: VM-Xa → VM-Xb (RabbitMQ, Temporal ports), VM-Xb → VM-2, VM-Xb → VM-3, VM-Xb/VM-Xa → github.com.
- Deliverable: matrix with pass/fail + timestamps.

## Acceptance criteria (done-when)

1. Four VMs up, Docker running, hello-container on each. ✔
2. RabbitMQ: cross-VM publish/consume works; topic + queue names documented.
3. Temporal: trivial workflow completes via CLI or SDK against the Postgres backend.
4. Network matrix passes 100% (failures recorded with remediation).
5. VM numbering/naming locked in the decision log; docs updated.

## Exit gate

M1 may start only when criteria 1–4 hold and topic naming (WP-0.4) is documented. A missed criterion is a chassis defect — fix in M0, don't carry it forward.

## Notes

- VM-2/VM-3 are provisioned in M0 but carry **placeholder** services only (M4 installs the real LLM-Wiki/Hindsight).
- Keep every provisioning step scriptable (one `deploy/` folder in the POC repo) so the chassis is rebuildable from scratch — the POC must be reproducible, not hand-built.
