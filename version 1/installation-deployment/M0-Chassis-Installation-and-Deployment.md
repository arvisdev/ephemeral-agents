# M0 Installation & Deployment — Chassis

**Companion:** `../delivery plans/M0-Chassis-Delivery-Plan.md` (what to deliver) · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | PVE host + 4 KVM VMs (vm-xa, vm-xb, vm-2, vm-3) |
| **Prereqs** | Proxmox 9.x host with PVE shell access; a DHCP/static IP pool; outbound Internet on the Proxmox LAN |
| **Hosts touched** | PVE host (VM creation), all 4 VMs (OS + Docker + services) |
| **Language (mandatory)** | All Temporal SDK code — workflows, workers, activities, adapters, CLI samples — **must be Go** (owner instruction: speed + size). No other language for Temporal code in this POC. |

## 0. Assumptions & conventions

- VMs run **Ubuntu 26.04 LTS** (matches the running Proxmox estate; use 24.04 if 26.04 images are unavailable).
- Hostnames: `vm-xa`, `vm-xb`, `vm-2`, `vm-3`; SSH key auth with user `poc`.
- All compose files, scripts, and generated `.env` (secrets) live in the POC repo under `deploy/m0/` — the chassis must be **reproducible from the repo**, not hand-built.
- Secrets (RabbitMQ/Postgres passwords) go in per-VM `.env` files, **gitignored** — never in compose files.
- Pin every image tag at deploy time and record the pins in `deploy/m0/image-tags.env`.

## 1. Provision the VMs (PVE shell, on the host)

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

```bash
sudo apt update && sudo apt install -y docker.io docker-compose-v2
sudo usermod -aG docker poc             # re-login after
docker run --rm alpine echo ok
docker compose version                  # confirm v2 plugin
```

## 3. RabbitMQ (Topic) on vm-xb — `deploy/m0/rabbitmq/compose.yaml`

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

## 6. Lock VM numbering (D-num) + repo layout

- Record in `deploy/m0/decision-log.md` (and update the open-items section of `../ARCHITECTURE.md`): (a) VM naming scheme (VM-Xa/Xb/2/3 as drawn, or renumbered); (b) **language decision: Go for all Temporal code** (owner instruction: speed + size).
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

| # | Check | Command / where |
|---|---|---|
| 1 | 4 VMs, Docker runs on each | `docker run --rm alpine echo ok` on all |
| 2 | Cross-VM topic publish/consume | §3 last block |
| 3 | Temporal healthy, `default` ns, **Go SDK smoke completes** | §4 + §4b |
| 4 | Network matrix 100% | §5, recorded |
| 5 | Numbering locked + docs updated | decision-log entry exists |

## 8. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| VMs unreachable on the LAN | bridge `vmbr0` membership; firewall on PVE (`/etc/network/interfaces` + `pve-firewall`); test `ping` from PVE host first |
| Temporal pod not ready | Postgres healthcheck failing → check `${TEMPORAL_DB_PASS}` in `.env`; first start can be slow (schema init) |
| Rabbit healthcheck flapping | memory alarm — bump `vm-xb` RAM or set a memory limit; check `rabbitmq-diagnostics` |
| Clock skew warnings in logs | `chrony` not enabled on all VMs (see §1) |
| Rebuild needed | `docker compose down -v` + wipe VM disks, re-run `deploy/m0/deploy.sh` — that's the point of the scriptable chassis |
