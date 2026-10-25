# M0 Installation & Deployment — Chassis

**Companion:** `../delivery plans/M0-Chassis-Delivery-Plan.md` (what to deliver) · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | PVE host + 4 KVM VMs (vm-xa, vm-xb, vm-2, vm-3) |
| **Prereqs** | Proxmox 9.x host with PVE shell access; a DHCP/static IP pool; outbound Internet on the Proxmox LAN |
| **Hosts touched** | PVE host (VM creation), all 4 VMs (OS + Docker + services) |
| **Language (mandatory)** | All Temporal SDK code — workflows, workers, activities, adapters, CLI samples — **must be Go** (owner instruction: speed + size). No other language for Temporal code in this POC. |

## 0. Assumptions & conventions

*Why:* pinning these up front is what makes the chassis **reproducible from the repo** — a rebuild is a scripted re-run, not tribal knowledge. Secrets live in gitignored per-VM `.env` files so they never enter compose files or git, and image pins in `image-tags.env` make every deployment byte-for-byte repeatable.

- VMs run **Ubuntu 26.04 LTS** (matches the running Proxmox estate; use 24.04 if 26.04 images are unavailable).
- Hostnames: `vm-xa`, `vm-xb`, `vm-2`, `vm-3`; SSH key auth with user `poc`.
- All compose files, scripts, and generated `.env` (secrets) live in the POC repo under `deploy/m0/` — the chassis must be **reproducible from the repo**, not hand-built.
- Secrets (RabbitMQ/Postgres passwords) go in per-VM `.env` files, **gitignored** — never in compose files.
- Pin every image tag at deploy time and record the pins in `deploy/m0/image-tags.env`.

## 1. Provision the VMs (PVE shell, on the host)

*Why:* stable VMIDs + hostnames are what every later script and doc references; **chrony is a hard dependency** — RabbitMQ and Temporal both misbehave under clock skew, and M2's reaper logic compares `poc.expires` timestamps across containers. The `poc` user is the single account all install scripts use.

```bash
# check existing VMIDs first, pick a free block (900-903 shown)
qm list
qm create 900 --name vm-xa --ostype l26 --memory 4096  --cores 2 --virtio-local 20 --net0 virtio,bridge=vmbr0 --agent enabled=1
qm create 901 --name vm-xb --ostype l26 --memory 8192 --cores 4 --virtio-local 40 --net0 virtio,bridge=vmbr0 --agent enabled=1
qm create 902 --name vm-2  --ostype l26 --memory 4096  --cores 2 --virtio-local 20 --net0 virtio,bridge=vmbr0 --agent enabled=1
qm create 903 --name vm-3  --ostype l26 --memory 4096  --cores 2 --virtio-local 20 --net0 virtio,bridge=vmbr0 --agent enabled=1
for i in 900 901 902 903; do qm start $i; done
```

Install Ubuntu on each (ISO/cloud-init per the estate's existing pattern). Post-boot on each VM:

```bash
sudo hostname vm-xa                      # per VM
# static IP via netplan (/etc/netplan/01-netcfg.yaml): vm-xa=10.0.90.x style from the PVE LAN pool
# chrony/NTP on ALL VMs (RabbitMQ and Temporal both depend on sane clocks):
sudo apt install -y chrony && sudo systemctl enable --now chrony
# create the deploy user + ssh keys:
sudo useradd -m -s /bin/bash poc && sudo usermod -aG sudo poc
```

## 2. Docker on every VM

*Why:* every later milestone ships as containers, so Docker on all four VMs is the minimal common denominator. Adding `poc` to the docker group lets the install scripts run without sudo — the junior-operator path.

```bash
sudo apt update && sudo apt install -y docker.io docker-compose-v2
sudo usermod -aG docker poc             # re-login after
docker run --rm alpine echo ok
docker compose version                  # confirm v2 plugin
```

## 3. RabbitMQ (Topic) on vm-xb — `deploy/m0/rabbitmq/compose.yaml`

*Why:* RabbitMQ (Topic) is the diagram's Manager ↔ RabbitMQ channel. The durable queue + binding + DLQ are the at-least-once backbone the M1 adapter builds on (the DLQ exists specifically for M1's poison-pill case). Binding the mgmt UI to loopback is the credential posture the whole POC keeps: no management plane ever opens to the LAN.

```yaml
services:
  rabbitmq:
    image: rabbitmq:4-management-alpine   # pin exact tag in image-tags.env
    hostname: rabbitmq
    ports: ["127.0.0.1:15672:15672"]      # mgmt UI loopback-only; 5672 via internal bridge
    environment:
      RABBITMQ_DEFAULT_USER: poc
      RABBITMQ_DEFAULT_PASS: ${RABBITMQ_PASS}   # from .env (gitignored, generated: openssl rand -hex 16)
    volumes: [rabbitmq-data:/var/lib/rabbitmq]
    healthcheck:
      test: ["CMD", "rabbitmq-diagnostics", "-q", "ping"]
      interval: 15s
volumes:
  rabbitmq-data: {}
```

Deploy + declare the topic topology (management API):

```bash
cd deploy/m0/rabbitmq && docker compose up -d
BASE=http://127.0.0.1:15672; AUTH="poc:$RABBITMQ_PASS"
curl -s -u "$AUTH" -XPOST "$BASE/api/exchanges/%2F" -d '{"name":"agent.tasks","type":"topic","durable":true}'
curl -s -u "$AUTH" -XPOST "$BASE/api/queues/%2F/agent.tasks.durable" -d '{"durable":true}'
curl -s -u "$AUTH" -XPOST "$BASE/api/bindings/%2F/e/agent.tasks/all/queue" -d '{"source":"agent.tasks","destination":"agent.tasks.durable"}'
curl -s -u "$AUTH" -XPOST "$BASE/api/queues/%2F/agent.tasks.dlq" -d '{"durable":true}'   # for M1 poison pills
```

Cross-VM publish test: from **vm-xa**, `rabbitmqadmin publish -x agent.tasks -r agent.tasks.durable -m hello` (or a curl to the mgmt API) → message count on the queue becomes 1 on vm-xb.

## 4. Temporal on vm-xb — `deploy/m0/temporal/compose.yaml`

*Why:* Temporal is what makes "ephemeral" safe — its retry/timeout/crash-recovery semantics are exactly what the M2 lifecycle leans on (a lost activity still gets reaped; a restarted engine never loses a started workflow). Its Postgres is **Temporal-only by rule**: M4's Hindsight gets its own instance on vm-3 so one database failure can never take down the other store.

```yaml
services:
  temporal:
    image: temporalio/server:1.27.0        # pin: check cli.temporal.io for current stable
    ports: ["7233"]
    environment:
      DB_HOST: postgres
      DB_PORT: "5432"
      DB_NAME: temporal
      DB_USER: temporal
      DB_PASSWORD: ${TEMPORAL_DB_PASS}
      DB_PLUGIN: postgres12
      DB_SEEDS: postgres
    depends_on: [postgres]
    healthcheck:
      test: ["CMD", "temporal", "operator", "cluster", "health"]
      interval: 15s
  postgres:
    image: postgres:16                     # pin
    environment:
      POSTGRES_DB: temporal
      POSTGRES_USER: temporal
      POSTGRES_PASSWORD: ${TEMPORAL_DB_PASS}
    volumes: [temporal-pg-data:/var/lib/postgresql/data]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U temporal"]
      interval: 10s
volumes:
  temporal-pg-data: {}
```

Deploy + verify:

```bash
cd deploy/m0/temporal && docker compose up -d
# Temporal CLI (build host or vm-xb):
sudo curl -sSL https://cli.temporal.io/install.sh | sudo bash
export PATH=$PATH:/usr/local/bin
temporal operator cluster health          # expect: service members HEALTHY
temporal operator namespace list          # shows: default
```

**4b. Go SDK smoke (the language-decision check)** — `deploy/m0/go-sdk-smoke/`: a minimal Go program, one `greet` workflow + worker on `github.com/temporalio/sdk-go`:

```bash
cd deploy/m0/go-sdk-smoke
go mod init go-sdk-smoke && go get github.com/temporalio/sdk-go@latest
go run . -addr vm-xb:7233              # start the workflow, run the worker, expect: "hello from greet"
```

This doubles as proof of the mandatory language decision: M0's gate includes "a Go-SDK workflow completes", and all later milestones build on this same Go toolchain.

Note: this Postgres belongs to Temporal only. VM-3's Hindsight gets its **own** Postgres in M4 — do not share instances.

## 5. Network verification matrix

*Why:* later milestones silently assume cross-VM name resolution and reachability; recording the full matrix — and *which* mechanism was chosen (PVE DNS vs `/etc/hosts`) in the decision log — turns an implicit assumption into an auditable entry. For the paper: this is the POC's environment baseline.

Run from each VM; record results in `deploy/m0/evidence/network-matrix.txt`:

```bash
# pairwise L3 reachability (VM-2/VM-3 have no services yet — probe ssh 22):
for h in vm-xa vm-xb vm-2 vm-3; do ssh -o ConnectTimeout=5 $h true && echo "$h OK"; done
# outbound Internet (needed for GitHub in later milestones):
for h in vm-xa vm-xb; do ssh $h 'curl -sI https://github.com | head -1'; done
# service ports on vm-xb:
ssh vm-xb 'nc -vz temporal 7233; nc -vz rabbitmq 5672'
```

For name resolution use PVE DNS, or add `/etc/hosts` lines on vm-xa/vm-xb/vm-2/vm-3 (the POC-internal choice; record which in the decision log).

## 6. Record the VM naming decision (D-8) + lock language + repo layout

*Why:* the language decision is the open item that, left open, gets re-litigated at every later milestone — locking it now costs ten minutes; locking it at M5 costs a re-hardening pass. VM naming (D-8) was resolved by the owner 2026-10-25 (labels are POC discussion devices, not commitments); this section just *records* that plus the POC's concrete hostnames. The repo layout is fixed here so the chassis is proven scriptable — `deploy.sh` exists to make "wipe and re-run" the recovery path.

- Record in `deploy/m0/decision-log.md` (and update the open-items section of `../ARCHITECTURE.md`): (a) **VM naming decision (D-8, owner 2026-10-25): the POC adopts the labels as drawn (VM-Xa/Xb/VM-2/VM-3) — these are POC discussion labels, not commitments; per-project naming is a deploy-time choice of the adopting team/company**; (b) the POC's concrete hostnames + which hostname maps to which label; (c) **language decision: Go for all Temporal code** (owner instruction: speed + size).
- Final repo layout for this milestone:

```
deploy/
  m0/
    rabbitmq/compose.yaml   temporal/compose.yaml
    image-tags.env          .env.example
    deploy.sh               # idempotent: creates everything from zero, safe to re-run
    evidence/network-matrix.txt
```

`deploy.sh` runs steps 2–5 in order with `set -euo pipefail` and a healthcheck gate after each service.

## 7. Acceptance checks (the M0 gate)

*Why:* these five checks are the M0 gate — the test procedure `TP-M0` formalizes them and must PASS before M1's installation may begin. The Go SDK smoke is check #3 precisely because the language mandate (all Temporal code in Go) has to be proven at the chassis level, not argued about later.

| # | Check | Command / where |
|---|---|---|
| 1 | 4 VMs, Docker runs on each | `docker run --rm alpine echo ok` on all |
| 2 | Cross-VM topic publish/consume | §3 last block |
| 3 | Temporal healthy, `default` ns, **Go SDK smoke completes** | §4 + §4b |
| 4 | Network matrix 100% | §5, recorded |
| 5 | Naming decision recorded (D-8) + docs updated | decision-log entry exists |

## 8. Troubleshooting

*Why:* the chassis must be proven **rebuildable** — this table exists so "wipe the VMs and re-run `deploy.sh`" is a documented recovery path, not a last resort. Each row pre-diagnoses a symptom so the operator goes straight to the fix.

| Symptom | Cause / fix |
|---|---|
| VMs unreachable on the LAN | bridge `vmbr0` membership; firewall on PVE (`/etc/network/interfaces` + `pve-firewall`); test `ping` from PVE host first |
| Temporal pod not ready | Postgres healthcheck failing → check `${TEMPORAL_DB_PASS}` in `.env`; first start can be slow (schema init) |
| Rabbit healthcheck flapping | memory alarm — bump `vm-xb` RAM or set a memory limit; check `rabbitmq-diagnostics` |
| Clock skew warnings in logs | `chrony` not enabled on all VMs (see §1) |
| Rebuild needed | `docker compose down -v` + wipe VM disks, re-run `deploy/m0/deploy.sh` — that's the point of the scriptable chassis |
