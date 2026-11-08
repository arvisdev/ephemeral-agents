# Chapter 14 — Building the Estate from Scratch

> **In plain terms.** Everything in this book ran on a real private cloud: a
> handful of virtual machines on a single server. This closing appendix-chapter
> answers the practical question — *"if I hand you a server, an account, and
> the names of the machines I want, can you build the whole thing?"* — with a
> yes, a parts list, and a timeline. It is the chapter a new operator reads
> before their first deployment.

## 14.1 What you supply (the complete list)

The build is automated, but automation still needs *inputs*. Here is the
complete intake, and why each item exists:

| # | Input | Why |
|---|---|---|
| 1 | Proxmox host address + an account with VM-creation rights | Everything is driven through the host's management interface |
| 2 | The names of the VMs to create | Becomes the estate inventory (see §14.2) |
| 3 | **An address plan** — which machine gets which address | The system is address-pinned by design; a plan filed at minute one prevents the drift class of failures (Chapter 12, C5) |
| 4 | An OS source — a cloud image, downloaded once | Bootstraps every VM without touching a console |
| 5 | The operator's SSH public key | Installed into every VM at birth; the password is used once, then retired |
| 6 | The model-provider key (e.g. an OpenRouter key) | Injected into worker environments at spawn; can't be conjured |
| 7 | Source-control credentials | A read token for the roster; and — if this is a brand-new organization — the ~5-minute manual creation of the org's application identity (the one step no automation can do, by the provider's own design; an existing identity reuses cleanly) |

Items 1–3 are decisions only the owner can make. Items 4–5 are one-time
plumbing. Items 6–7 are secrets: they are staged into permission-restricted
files on the machines that need them, never into images, never into the
repository — the credential discipline of Chapter 8 applies from minute one.

## 14.2 First artifact: the estate inventory

Before any machine exists, one file is written and committed:

```
# hosts.env — the estate inventory (single source of truth)
PVE_HOST=<pve-host-address>
VMID_1=<execution-host-vmid>   NAME_1=<execution-host>   ADDR_1=<execution-address>
VMID_2=<front-host-vmid>       NAME_2=<front-host>       ADDR_2=<front-address>
VMID_3=<memory-host-vmid>      NAME_3=<memory-host>      ADDR_3=<memory-address>
VMID_4=<registry-host-vmid>    NAME_4=<registry-host>    ADDR_4=<registry-address>
```

This file is the build's anchor — and the lesson that created it is worth
retelling: during the original build, the estate changed underneath the
system (a machine was replaced; every address was reassigned), and the
mismatch between inventory and reality was discovered *three construction
stages deep*. The rule that followed: the inventory is written first,
committed, and every subsequent stage validates against it. The very first
build step is therefore a **dry-run check** (`--check-only`) that compares
the inventory to the real host — catching a wrong machine name or address
at minute five instead of hour five.

## 14.3 Creating the machines

For each VM in the inventory:

1. **Create** — allocate disk, memory, and cores from the sizing table
   (the memory host wants the most: its vector database is sized at
   40+ GB of disk; the execution host carries the worker images, including
   a ~3 GB executor).
2. **Install the OS without a console.** Two proven paths:
   - **Cloud image + cloud-init (recommended for greenfield):** the VM boots
     a pre-built server image that reads a small config file at first boot —
     address, hostname, SSH key, done. Fully unattended, repeatable.
   - **Offline installation (the repair pattern):** attach the VM's disk to
     the host, prepare the OS on it directly, detach, boot. This is how the
     original build recovered a replaced machine; it works, but it is
     surgery — use it when a cloud image is not available.
3. **First boot baseline** (scripted, ~minutes per machine): static address
   per the inventory, the operator's SSH key installed, an unprivileged
   operations account with passwordless sudo (its exact grants are recorded
   for later revocation), the container runtime installed, and the host's
   `/etc/hosts` entry set.
4. **Verify** — the chassis check (`--check-only`) re-run: every machine
   answers over SSH, runs containers, and matches the inventory.

At this point the estate exists but is empty — four machines with an
operating system and nothing else. Everything from here is the milestone
pipeline.

## 14.4 The deployment sequence

The stages are the same ones the original build used and gated (Chapter 11),
and every install script is **idempotent**: re-running against a living
estate re-verifies what is green and resumes at the first failure.

```
Stage 1 (M0)  chassis: message broker topology, workflow engine + store,
              cross-machine publish proof, decision log
Stage 2 (M1)  message bridge: topic exchange, durable queue, consumer proof
Stage 3 (M2)  durable core: task workflow, lifecycle, reaper;
              crash-recovery, deadline, and mixed-outcome gates
Stage 4 (M3)  roster + executors + credentials:
              persona hydration, failure paths, real repository edits,
              token mint/revocation proof, concurrency, secret scans
Stage 5 (M4)  memory: vector store + memory API, read/write contracts,
              cross-task inheritance proof
Stage 6 (M5)  front door + whole system: gateway, registrar,
              stage-gated bring-up, the end-to-end acceptance run,
              failure injections, hardening, open-item lockout
```

The last stage's exit criterion is the book's verdict in miniature: an
acceptance request travels the full path — human → front door → work order →
worker → real, verified artifact → report — and the run ends with **zero
surviving containers** and every gate green.

## 14.5 Secrets staging (the one part that is deliberately manual-ish)

Secrets are never committed and never baked into images. They are staged
into permission-restricted files on the specific machines that consume them:

- the roster read token and model key → the execution host's secrets file;
- the credential issuer's private key → the execution host only, mode 0600;
- message-broker and database credentials → generated at install time into
  gitignored files.

A post-deployment hygiene pass closes the loop: secret-pattern scans across
every built image, an environment sweep of every running container, and a
git-history check — the same gates the original build ran (Chapter 8).

## 14.6 Timeline, honestly

| Phase | Time |
|---|---|
| Inventory + dry-run | ~15 min |
| VM creation + OS + baseline (×4, scripted) | ~1 hour |
| Stage 1–2 installs + gates | ~30 min |
| Stage 3 installs + gates (includes image builds) | ~1–2 hours |
| Stage 4 install + gates | ~30 min |
| Stage 5 install + gates (includes Go builds + the 3 GB image) | ~1–2 hours |
| **Total, greenfield** | **~half a day** |

A rebuild onto *existing* machines (the idempotent path) is faster still —
the original build re-ran whole stages against a live, changed estate in
minutes, which is exactly why the idempotency rule exists.

## 14.7 The three rules for the operator

1. **Inventory first, dry-run first.** Never let the first real command be
   a provisioning command. (Chapter 12, class C5.)
2. **Passwords retire immediately.** The host password provisions the SSH
   key and is then used by no one; all subsequent access is key-only, and
   the credential files that do exist are permission-restricted and
   gitignored.
3. **Nothing load-bearing lives off the repository.** If a deployment step
   produced a working config from a hand edit, that edit goes into source
   control before the deployment is called done — otherwise the next
   rebuild silently loses it (Chapter 12, Lesson L3).

## 14.8 Closing the loop

With the estate standing, the system described in Parts I and II is
*running*: the roster is live, workers can be spawned, credentials mint and
die, memory accumulates, and the front door accepts work. The build from
scratch is deliberately unremarkable — four commands' worth of stages, each
gated, each re-runnable — because that is the point: **the organization is
provisionable the way software is**, from source, with gates, by anyone who
can follow this chapter.
