# Deploy — install scripts for the ephemeral-agents POC

One install script per milestone (`m0/` … `m5/`), plus this README and shared
state. **This folder is where you start if you have been handed this POC and
told to "run the install."**

## What you are looking at

```
deploy/
├── README.md                      <- you are here
├── m0/..m5/
│   ├── install.sh                 <- the install script for that milestone
│   ├── evidence/                  <- where gate results land (created at run)
│   └── *.env / compose files      <- per-milestone config (gitignored where secret)
```

Each `install.sh` does three things, in order:

1. **Check preconditions** (`--check-only` runs only this).
2. **Install + deploy** that milestone's pieces (idempotent — re-running skips
   what's already done).
3. **Run the milestone's gate tests** and record results in `evidence/`.

## The sequence (this is the whole POC, in one picture)

| Step | Do | Result you need before the next step |
|---|---|---|
| 0 | Read the M0 install doc, `cp deploy/m0/hosts.env.example deploy/m0/hosts.env` and fill it | — |
| 1 | `bash deploy/m0/install.sh` | M0 gate = 5 PASS → run **TP-M0** |
| 2 | `bash deploy/m1/install.sh` | T1–T5 PASS → run **TP-M1** |
| 3 | `bash deploy/m2/install.sh` | V1–V5 PASS → run **TP-M2** |
| 4 | `bash deploy/m3/install.sh` | B1–B5 PASS → run **TP-M3** |
| 5 | `bash deploy/m4/install.sh` | G1–G6 PASS → run **TP-M4** |
| 6 | `bash deploy/m5/install.sh` | E2E + F1–F4 + hardening → run **TP-M5** |

**The rule (the gate):** a milestone's test procedure (`version 1/test
procedures/TP-M<n>`) must be signed **before** the next install script is
started. The scripts probe this for you (M0_DONE etc.) — but the *sign-off*
is the human's job.

## First time on this POC? Do these once, then start at M0.

On the **build workstation**:

- docker (+ buildx), go, the `temporal` CLI, `gh` (logged in), rsync,
  an SSH keypair → authorize the public half for the `poc` user on PVE + the
  4 VMs (the scripts SSH as `poc` everywhere, with BatchMode/key auth only;
  `poc` needs **passwordless sudo** on the VMs — m0/m4/m5 provisioning steps
  use it).
- `ssh -o StrictHostKeyChecking=accept-new poc@PVE_HOST` once per host —
  known_hosts gets seeded for you.
- `mkdir -p /opt/poc/deploy/m1 /opt/poc/deploy/m2 /opt/poc/deploy/m3
  /opt/poc/deploy/m5` on the VMs that need them (the scripts use
  `/opt/poc/deploy/m<n>` and create the compose dirs themselves; these are
  the parents).

## When a step fails, where do you look?

Each script prints the owning install-doc section for any failure. The routing
rule is:

- failure names a **section (n)** → open that milestone's install doc §n,
  follow its Troubleshooting row.
- a **gate test** fails → that milestone's install doc §4/§6 has the fix;
  re-run *from the failed test* (the scripts support this — see each
  operator block).
- M5 E2E fails at stage **Sn** → that stage names its owning milestone;
  re-run *that* milestone's gate test, not the whole chain.

You never need to re-run a previous milestone's install script to fix a later
one — each script is scoped to its own milestone.

## Common flags (every script)

| Flag / env | Effect |
|---|---|
| `--check-only` | run preconditions only, touch nothing |
| `ASSUME_YES=1` | skip the interactive "proceed?" prompt (CI / non-interactive) |
| `M0_DONE=1 M1_DONE=1 …` | skip the "is the previous milestone green?" probes (set after a signed TP) |
| `--no-gate` | skip the gate-test section (rare; use only when re-running tests by hand) |

## What lands where

| Artifact | Location |
|---|---|
| Per-milestone gate results | `deploy/m<n>/evidence/` (created at run) |
| Temporal history for the POC | Temporal Web on vm-xb, namespace `default` (no `poc` namespace is created) |
| Docker state (images, containers) | on the VMs (vm-xb: rabbitmq + temporal + their postgres; vm-hindsight: hindsight; vm-llm-wiki: llmwiki; vm-xa: front-of-house at M5) |
| GitHub App key (M3) | `/opt/poc/secrets/app-key.pem` on **vm-xb** (chmod 600, never in git) — where `issue-token.sh` runs |

## The POC is complete when (from the M5 install doc §9)

1. E2E run passed, **zero** surviving POC containers
2. All four failure injections clean + reported to the human
3. Security scans pass; no long-lived owner secret reachable from any container
4. The four Version 2 open items are *listed* (not solved)
5. `TP-M5` is signed

When those hold, the feasibility-analysis verdict is confirmed **by evidence,
not by analysis** — that sentence is the POC's closing line, and these
scripts are how you get to it.
