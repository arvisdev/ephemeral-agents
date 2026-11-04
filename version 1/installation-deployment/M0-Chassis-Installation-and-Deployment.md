# M0 Installation & Deployment — Infrastructure Foundation

**Companion:** `../delivery plans/M0-Chassis-Delivery-Plan.md` (what to deliver) · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | PVE host + 3 KVM VMs (vm-xa, vm-xb, vm-hindsight) — *(D-wiki-git, 2026-11-04: vm-llm-wiki / VMID 902 is dropped; the POC is a 3-VM topology, reference standards live in the git repo `github.com/arvisdev/llm-wiki`)* |
| **Prereqs** | Proxmox 9.x host with PVE shell access; a DHCP/static IP pool; outbound Internet on the Proxmox LAN |
| **Hosts touched** | PVE host (VM creation), all 3 VMs (OS + Docker + services) |
| **Language (mandatory)** | All Temporal SDK code — workflows, workers, activities, adapters, CLI samples — **must be Go** (owner instruction: speed + size). No other language for Temporal code in this POC. |

## 0. Assumptions & conventions

*Why:* pinning these up front is what makes the chassis **reproducible from the repo** — a rebuild is a scripted re-run, not tribal knowledge. Secrets live in gitignored per-VM `.env` files so they never enter compose files or git, and image pins in `image-tags.env` make every deployment byte-for-byte repeatable.

- VMs run **Ubuntu 26.04 LTS** (matches the running Proxmox estate; use 24.04 if 26.04 images are unavailable).
- Hostnames: `vm-xa`, `vm-xb`, `vm-hindsight`; SSH key auth with user `poc`. *(2026-11-04, D-wiki-git: `vm-llm-wiki` is no longer provisioned; the command blocks below predate the decision — create only vm-xa, vm-xb, vm-hindsight, i.e. skip VMID 902 and any step naming vm-llm-wiki.)*
- All compose files, scripts, and generated `.env` (secrets) live in the POC repo under `deploy/m0/` — the chassis must be **reproducible from the repo**, not hand-built.
- Secrets (RabbitMQ/Postgres passwords) go in per-VM `.env` files, **gitignored** — never in compose files.
- Pin every image tag at deploy time and record the pins in `deploy/m0/image-tags.env`.

## 1. Provision the VMs (PVE shell, on the host)

*Why:* stable VMIDs + hostnames are what every later script and doc references; **chrony is a hard dependency** — RabbitMQ and Temporal both misbehave under clock skew, and M2's reaper logic compares `poc.expires` timestamps across containers. The `poc` user is the single account all install scripts use.

```bash
# check existing VMIDs first, pick a free block (900-903 shown)
qm list
qm create 900 --name vm-xa --ostype l26 --memory 4096  --cores 2 --virtio0 local:20 --net0 virtio,bridge=vmbr0 --agent enabled=1
qm create 901 --name vm-xb --ostype l26 --memory 8192 --cores 4 --virtio0 local:40 --net0 virtio,bridge=vmbr0 --agent enabled=1
qm create 902 --name vm-llm-wiki  --ostype l26 --memory 4096  --cores 2 --virtio0 local:20 --net0 virtio,bridge=vmbr0 --agent enabled=1   # D-wiki-git 2026-11-04: DO NOT create — VM-2 dropped (3-VM topology; wiki = git repo arvisdev/llm-wiki)
qm create 903 --name vm-hindsight  --ostype l26 --memory 4096  --cores 2 --virtio0 local:20 --net0 virtio,bridge=vmbr0 --agent enabled=1
for i in 900 901 902 903; do qm start $i; done
```

### 1a. Install the OS (the `qm create` disks start EMPTY)

> **Owner-provisioned baseline (2026-10-30):** if the three VMs arrive already provisioned — clean Ubuntu 26.04 LTS, all updates applied, SSH enabled and allowed through UFW — **skip Path A/B entirely** and go straight to the post-boot block below. The baseline does *not* include: hostname/netplan, chrony, the `poc` user (create it; the scripts SSH as `poc` with key auth and need passwordless sudo), Docker (section 2), or the UFW service-port rules — the latter the **install scripts open themselves** (m0 section 2a, m4, m5), so nothing needs hand-editing.

*Why:* `qm create --virtio0 local:20` allocates a blank disk — nothing boots from it. A literal follower of the block above gets unbootable VMs, so this step is not optional: attach an OS image, install (or cloud-init) Ubuntu as user `poc`, and only then do the post-boot steps below apply. The estate's existing pattern is an Ubuntu cloud-init image; if you have no template, fall back to the ISO + manual install path (both are given). *(Three VMs only per D-wiki-git — the blocks still show 900–903; skip 902.)*

**Path A — cloud-init template (preferred):** on the PVE host, attach the estate's Ubuntu cloud-init image/template to each VM and seed the `poc` user + SSH key + hostname via cloud-init:

```bash
# once, if no template exists yet: download + import the Ubuntu cloud image into local storage
# (matches the estate's Ubuntu 26.04 LTS assumption; 24.04 image if 26.04 is unavailable)
wget https://cloud-images.ubuntu.com/<release>/current/<release>-server-cloudimg-amd64.img -P /var/lib/vz/template/iso/
for i in 900 901 902 903; do
  qm importdisk $i /var/lib/vz/template/iso/<release>-server-cloudimg-amd64.img local
  qm set $i --scsihw virtio-scsi-pci --scsi0 local:vm-$i-disk-1      # attach the imported disk as boot disk
  qm set $i --boot order=scsi0
  qm set $i --ide2 local:cloudinit                                   # cloud-init drive
  qm set $i --ciuser poc --sshkeys ~/.ssh/id_ed25519.pub             # deploy user + key auth (matches M0's ssh-as-poc rule)
  qm set $i --cipassword "$(openssl rand -hex 8)"                    # throwaway; SSH-key auth is the real path
  qm set $i --hostname vm-xa                                         # per VM: 900=vm-xa 901=vm-xb 903=vm-hindsight (902/vm-llm-wiki dropped, D-wiki-git 2026-11-04)
  qm reboot $i                                                       # re-boot into the installed disk
done
```

**Path B — ISO + manual install:** upload an Ubuntu ISO to `local` (`ISO Images → Upload` in the PVE web UI or `qm importdisk`-equivalent `pvesm`), then per VM:

```bash
qm set <vmid> --ide2 local:iso/<ubuntu-iso>.iso,media=cdrom --boot order=ide2
qm start <vmid>
# open the console (noVNC), walk the installer: hostname as above, user 'poc'
# (add to sudo group), erase-disk install onto the 20/40G virtio disk
qm set <vmid> --ide2 none,media=cdrom                               # eject the ISO after install
```

Either path lands you at the same state the post-boot block below assumes: Ubuntu booted on the VM, a `poc` user present (cloud-init path creates it for you — skip the `useradd` in the post-boot block if so), ready for hostname/netplan/chrony. The static IPs you set via netplan **must be the same ones you write into `deploy/m0/hosts.env` (NETPLAN_IP_*) and your name-resolution mechanism** — see `deploy/m0/hosts.env.example` for the worked example and that rule.

Post-boot on each VM:

```bash
sudo hostname vm-xa                      # per VM
# static IP via netplan (/etc/netplan/01-netcfg.yaml): vm-xa=10.0.90.x style from the PVE LAN pool
# chrony/NTP on ALL VMs (RabbitMQ and Temporal both depend on sane clocks):
sudo apt install -y chrony && sudo systemctl enable --now chrony
# create the deploy user + ssh keys (skip useradd if cloud-init already created poc):
sudo useradd -m -s /bin/bash poc && sudo usermod -aG sudo poc
# UFW: SSH is already allowed (owner baseline). **The install scripts open the
# POC's cross-VM service ports themselves** — m0 section 2a opens 5672/7233 on vm-xb,
# the M4 script opens 8081 on vm-hindsight (no wiki VM since D-wiki-git,
# 2026-11-04), the M5 script
# opens 8080 (gateway) on vm-xa — all scoped to the POC LAN derived from
# hosts.env's NETPLAN_IP_* (override with UFW_POC_CIDR; the gateway's source
# is GATEWAY_ALLOW_CIDR since the engineer's desktop is often outside the
# LAN). Nothing to hand-edit here; the manual fallback (if you must run
# without the scripts) is the four rules below:
sudo ufw allow from <poc-lan-cidr> to any port 5672 proto tcp   # vm-xb only
sudo ufw allow from <poc-lan-cidr> to any port 7233 proto tcp   # vm-xb only
sudo ufw allow from <poc-lan-cidr> to any port 8080 proto tcp   # vm-xa only (D-wiki-git 2026-11-04: vm-llm-wiki dropped — no wiki VM/service exists)
sudo ufw allow from <poc-lan-cidr> to any port 8081 proto tcp   # vm-hindsight only
```

## 2. Docker on every VM

*Why:* every later milestone ships as containers, so Docker on all three VMs is the minimal common denominator. Adding `poc` to the docker group lets the install scripts run without sudo — the junior-operator path.

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
    ports:
      - "5672:5672"                        # AMQP must be reachable cross-VM (M5 manager on vm-xa publishes here)
      - "127.0.0.1:15672:15672"            # mgmt UI loopback-only — no management plane on the LAN
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
curl -s -u "$AUTH" -XPOST "$BASE/api/exchanges/%2F/agent.tasks" -d '{"type":"topic","durable":true}'
curl -s -u "$AUTH" -XPOST "$BASE/api/queues/%2F/agent.tasks.durable" -d '{"durable":true}'
curl -s -u "$AUTH" -XPOST "$BASE/api/bindings/%2F/e/agent.tasks/q/agent.tasks.durable" -d '{"routing_key":"agent.tasks.#"}'   # per D-routing: wildcard binding — V2 publishers need no re-binding
curl -s -u "$AUTH" -XPOST "$BASE/api/queues/%2F/agent.tasks.dlq" -d '{"durable":true}'   # for M1 poison pills
curl -s -u "$AUTH" -XPOST "$BASE/api/queues/%2F/agent.tasks.response" -d '{"durable":true}'  # M5 manager consumes; M2 worker publishes results
```

Cross-VM publish test: from **vm-xa**, verify AMQP reachability (`nc -vz -w3 vm-xb 5672`), then publish through the exchange (from vm-xa with any AMQP client against vm-xb:5672, or locally on vm-xb via the mgmt API) → message count on the queue becomes 1 on vm-xb. (The mgmt UI stays loopback-only; AMQP 5672 is published cross-VM because M5's manager on vm-xa publishes to it.)

## 4. Temporal on vm-xb — `deploy/m0/temporal/compose.yaml`

*Why:* Temporal is what makes "ephemeral" safe — its retry/timeout/crash-recovery semantics are exactly what the M2 lifecycle leans on (a lost activity still gets reaped; a restarted engine never loses a started workflow). Its Postgres is **Temporal-only by rule**: M4's Hindsight gets its own instance on vm-hindsight so one database failure can never take down the other store.

```yaml
services:
  temporal:
    image: temporalio/auto-setup:1.27.0    # pin; auto-setup runs the schema init the bare server image lacks
    ports: ["7233"]
    environment:
      DB: postgres12
      DB_PORT: "5432"
      POSTGRES_USER: temporal
      POSTGRES_PWD: ${TEMPORAL_DB_PASS}
      POSTGRES_SEEDS: postgres
    depends_on: [postgres]
    healthcheck:
      test: ["CMD-SHELL", "bash -c '</dev/tcp/127.0.0.1/7233'"]  # server image ships no temporal CLI
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

Prereq: a pinned Go toolchain (≥ 1.25) on the build host — `go version` must succeed before this step.

This doubles as proof of the mandatory language decision: M0's gate includes "a Go-SDK workflow completes", and all later milestones build on this same Go toolchain.

Note: this Postgres belongs to Temporal only. VM-3's Hindsight gets its **own** Postgres in M4 — do not share instances.

## 5. Network verification matrix

*Why:* later milestones silently assume cross-VM name resolution and reachability; recording the full matrix — and *which* mechanism was chosen (PVE DNS vs `/etc/hosts`) in the decision log — turns an implicit assumption into an auditable entry. For the paper: this is the POC's environment baseline.

Run from each VM; record results in `deploy/m0/evidence/network-matrix.txt`:

```bash
# pairwise L3 reachability (VM-3 has no services yet — probe ssh 22):  # D-wiki-git 2026-11-04: VM-2 dropped — 3-VM topology
for h in vm-xa vm-xb vm-hindsight; do ssh -o ConnectTimeout=5 $h true && echo "$h OK"; done
# outbound Internet (needed for GitHub in later milestones):
for h in vm-xa vm-xb; do ssh $h 'curl -sI https://github.com | head -1'; done
# service ports on vm-xb:
ssh vm-xb 'nc -vz temporal 7233; nc -vz rabbitmq 5672'
```

For name resolution use PVE DNS, or add `/etc/hosts` lines on vm-xa/vm-xb/vm-hindsight (the POC-internal choice; record which in the decision log). *(vm-llm-wiki is gone per D-wiki-git, 2026-11-04 — the loop above predates the decision; drop it from the host list.)*

## 6. Record the VM naming decision (D-8) + lock language + repo layout

*Why:* the language decision is the open item that, left open, gets re-litigated at every later milestone — locking it now costs ten minutes; locking it at M5 costs a re-hardening pass. VM naming (D-8) was resolved by the owner 2026-10-25 (labels are POC discussion devices, not commitments); this section just *records* that plus the POC's concrete hostnames. The repo layout is fixed here so the chassis is proven scriptable — `install.sh` exists to make "wipe and re-run" the recovery path.

- Record in `deploy/m0/decision-log.md` (and note the resolutions in the open-items section of `../Initial-Foundation-Design-Document.md` section 7): (a) **VM naming decision (D-8, owner 2026-10-25): the POC adopts the labels as drawn (VM-Xa/Xb/VM-2/VM-3) — these are POC discussion labels, not commitments; per-project naming is a deploy-time choice of the adopting team/company**; (b) the POC's concrete hostnames, mapped to their labels: `vm-xa`→VM-Xa, `vm-xb`→VM-Xb, `vm-llm-wiki`→VM-2 (LLM-Wiki), `vm-hindsight`→VM-3 (Hindsight); (c) **language decision: Go for all Temporal code** (owner instruction: speed + size).
- Final repo layout for this milestone:

```
deploy/
  m0/
    rabbitmq/compose.yaml   temporal/compose.yaml
    image-tags.env          .env.example
    install.sh              # idempotent: creates everything from zero, safe to re-run
    evidence/network-matrix.txt
```

`install.sh` runs steps 2–5 in order with `set -euo pipefail` and a healthcheck gate after each service (re-runs skip what's already done).

## 7. Acceptance checks (the M0 gate)

*Why:* these five checks are the M0 gate — the test procedure `TP-M0` formalizes them and must PASS before M1's installation may begin. The Go SDK smoke is check #3 precisely because the language mandate (all Temporal code in Go) has to be proven at the chassis level, not argued about later.

| # | Check | Command / where |
|---|---|---|
| 1 | 4 VMs, Docker runs on each | `docker run --rm alpine echo ok` on all |
| 2 | Cross-VM topic publish/consume | section 3 last block |
| 3 | Temporal healthy, `default` ns, **Go SDK smoke completes** | section 4 + section 4b |
| 4 | Network matrix 100% | section 5, recorded |
| 5 | Naming decision recorded (D-8) + docs updated | decision-log entry exists |

## 8. Troubleshooting

*Why:* the chassis must be proven **rebuildable** — this table exists so "wipe the VMs and re-run `install.sh`" is a documented recovery path, not a last resort. Each row pre-diagnoses a symptom so the operator goes straight to the fix.

| Symptom | Cause / fix |
|---|---|
| VMs unreachable on the LAN | bridge `vmbr0` membership; firewall on PVE (`/etc/network/interfaces` + `pve-firewall`); test `ping` from PVE host first |
| Temporal pod not ready | Postgres healthcheck failing → check `${TEMPORAL_DB_PASS}` in `.env`; first start can be slow (schema init) |
| Rabbit healthcheck flapping | memory alarm — bump `vm-xb` RAM or set a memory limit; check `rabbitmq-diagnostics` |
| Clock skew warnings in logs | `chrony` not enabled on all VMs (see section 1) |
| Rebuild needed | `docker compose down -v` + wipe VM disks, re-run `deploy/m0/install.sh` — that's the point of the scriptable chassis |
