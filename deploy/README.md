# Deploy — install scripts for the ephemeral-agents POC

One install script per milestone (`m0/` … `m5/`), plus this README and shared
state. **This folder is where you start if you have been handed this POC and
told to "run the install."**

## What you are looking at

```
deploy/
├── README.md                      <- you are here
├── m0/ … m5/                      <- one directory per milestone
│   ├── install.sh                 <- the install script for that milestone
│   ├── evidence/                  <- where gate results land (created at run)
│   └── *.env / compose files      <- per-milestone config (gitignored where secret)
```

Each `install.sh` does three things, in order:

1. **Check preconditions** (`--check-only` runs only this).
2. **Install + deploy** that milestone's pieces (idempotent — re-running skips
   what's already done).
3. **Run the milestone's gate tests** and record results in `evidence/`.

## Zero → done: the full path, in order

This is the walk-through for someone starting from an empty desk. Do the
steps **in order**; each names exactly what to read and what to run.

### Stage A — before you touch anything (half a day)

1. Read this file top to bottom (it is the map; the milestone docs are the
   terrain).
2. Collect the owner-inputs below ("Owner must hand you these") — all of
   them, before starting. Missing item ⇒ stop and ask; do not improvise.
3. Confirm the four VMs exist with the owner baseline (clean Ubuntu 26.04,
   updated, SSH through UFW — see M0 doc §1a's baseline note).
4. `cp deploy/m0/hosts.env.example deploy/m0/hosts.env` and fill it (ssh
   aliases + the four static IPs; same IPs in netplan/DNS — the file's own
   comments explain the rule).
5. `bash deploy/check-workstation.sh` — fix everything it flags (tools,
   SSH key auth as `poc`, passwordless sudo) until it is quiet.

### Stage B — M0, the chassis (day 1)

1. Read `version 1/installation-deployment/M0-Chassis-Installation-and-Deployment.md`
   §0–§8 once. It is the only milestone where you create infrastructure.
2. `bash deploy/m0/install.sh --check-only` → then `bash deploy/m0/install.sh`.
3. Run TP-M0's 8 cases, sign it. **Nothing else starts until TP-M0 is signed.**

### Stage C — M1…M4, the components (the bulk of the work)

For each milestone in order (M1 adapter → M2 lifecycle → M3 executors →
M4 memory/knowledge):

1. Read that milestone's install doc fully, then its delivery plan if you
   need the *why* behind a choice.
2. `bash deploy/m<n>/install.sh --check-only` → `bash deploy/m<n>/install.sh`.
3. Run that milestone's TP cases; sign before the next milestone.

Pacing notes: M2's V4 (crash recovery) is the load-bearing test — if it
cannot pass, STOP (the script tells you too). M3 needs the owner's GitHub
App; M4 needs `CONTENT_REPO`/`LLMWIKI_READ_TOKEN`/`EMBEDDING_PROVIDER`.

### Stage D — M5, end-to-end (the finale)

1. Read the M5 install doc; `--check-only` → run it. It brings up the
   front-of-house, runs the ordered bring-up (S1–S7), one real E2E task,
   four failure injections, and the hardening scans.
2. Run TP-M5 and sign it.

### Stage E — declare completion

Check the five conditions in "The POC is complete when" below. All five
⇒ the POC is done; hand the evidence trail to the owner.

Rule of thumb for the whole journey: **read before you run, `--check-only`
before you run for real, sign before you move on, and when anything fails,
the "When a step fails" section below tells you where the fix lives.**

## The sequence (this is the whole POC, in one picture)

| Step | Do | Result you need before the next step |
|---|---|---|
| 0 | Read the M0 install doc, `cp deploy/m0/hosts.env.example deploy/m0/hosts.env` and fill it | — |
| 1 | `bash deploy/m0/install.sh` | M0 gate (5 checks §7) PASS → run **TP-M0** (8 mandatory cases) |
| 2 | `bash deploy/m1/install.sh` | T1–T5 PASS → run **TP-M1** (6 mandatory cases) |
| 3 | `bash deploy/m2/install.sh` | V1–V5 PASS → run **TP-M2** (6 mandatory cases) |
| 4 | `bash deploy/m3/install.sh` | B1–B5(+B6) PASS → run **TP-M3** (7 mandatory cases) |
| 5 | `bash deploy/m4/install.sh` | G1–G6 PASS → run **TP-M4** (7 mandatory cases) |
| 6 | `bash deploy/m5/install.sh` | E2E + F1–F4 + hardening → run **TP-M5** |

The script gate labels (§-checks, T/V/B/G, F) are the script's own block
names; the **TP case counts are the authoritative gate** — sign off against
the test procedure, not the script output alone.

**The rule (the gate):** a milestone's test procedure (`version 1/test
procedures/TP-M<n>`) must be signed **before** the next install script is
started. The scripts probe this for you (M0_DONE etc.) — but the *sign-off*
is the human's job.

## Owner must hand you these (before you touch any script)

*The POC needs inputs only the owner can supply. Collect them FIRST — every
milestone below assumes they exist, and hunting for them mid-install is how a
junior run stalls.* If any item is missing, stop and ask before starting M0.

- [ ] **The four VMs, pre-provisioned** — clean Ubuntu 26.04 LTS, all updates
      applied, SSH enabled and allowed through UFW (owner baseline,
      2026-10-30). Everything else on the VMs (hostname/netplan, chrony, the
      `poc` user, Docker) is M0's job; the UFW service-port rules are opened
      automatically by the install scripts (m0 §2a / m4 / m5).
- [ ] **M3: GitHub App + private key** — the App ID + the key file, to be
      placed at `/opt/poc/secrets/app-key.pem` on **vm-xb** (chmod 600, never
      in git). Needed by `deploy/m3/issue-token.sh`.
- [ ] **Available Agent List repo + read token** — the repo the agent index is
      read from, plus a read-only token the gateway can use. Layout contract:
      `version 1/Available-Agent-List-Repository.md`; if the repo doesn't
      exist yet, create it from the working example
      `deploy/m5/agent-list-example/`.
- [ ] **M4 content settings: `CONTENT_REPO` + `LLMWIKI_READ_TOKEN`** — the
      LLM-Wiki content repo and its read token (vm-llm-wiki).
- [ ] **`EMBEDDING_PROVIDER` + API key** — which embedding backend the POC
      uses and its key (same model for ingest and query in M4).
- [ ] **TTL / cap configuration values** — `POC_TTL_DEFAULT` and the team cap
      (agents per team / concurrent-run limits) the M2 reaper and M5 manager
      enforce.
- [ ] **Network facts** — the PVE LAN IP pool to draw the four static VM IPs
      from, and the decision on name resolution (PVE DNS vs `/etc/hosts`) to
      record in `deploy/m0/decision-log.md`.

## First time on this POC? Do these once, then start at M0.

> **Operating via an AI agent (Hermes Desktop):** the intended junior-operator
> path is to work with an AI agent on the operator's Hermes Desktop, which
> runs these same scripts over SSH — nothing in this POC is hand-typed into
> the VMs. The agent needs exactly what's listed below (SSH key auth as
> `poc`, passwordless sudo on the VMs) and should run each script with
> `--check-only` first; the scripts print their owning doc section on any
> failure, which is what the agent follows to recover.

On the **build workstation**:

- docker (+ buildx), go, the `temporal` CLI, `gh` (logged in), rsync,
  an SSH keypair → authorize the public half for the `poc` user on PVE + the
  4 VMs (the scripts SSH as `poc` everywhere, with BatchMode/key auth only;
  `poc` needs **passwordless sudo** on the VMs — used by m0's VM provisioning/netplan and m4's `/etc/hosts` automation; the common container path (docker compose, docker CLI) runs sudo-free via the `poc`-in-docker-group setup in M0 §2).
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
| `--no-gate` | run the install but don't fail the script on gate-test failures (the gate section still runs and still writes evidence — use only when re-running tests by hand) |

## What lands where

| Artifact | Location |
|---|---|
| Per-milestone gate results | `deploy/m<n>/evidence/` (created at run) |
| Temporal history for the POC | `temporal workflow list/show` via the CLI on vm-xb, namespace `default` (no Web UI is deployed; no `poc` namespace is created) |
| Docker state (images, containers) | on the VMs (vm-xb: rabbitmq + temporal + their postgres; vm-hindsight: hindsight; vm-llm-wiki: llmwiki; vm-xa: front-of-house at M5) |
| GitHub App key (M3) | `/opt/poc/secrets/app-key.pem` on **vm-xb** (chmod 600, never in git) — where `issue-token.sh` runs |

## The POC is complete when (from the M5 install doc §9)

1. E2E run passed, **zero** surviving POC containers
2. All four failure injections clean + reported to the human
3. Security scans pass; no long-lived owner secret reachable from any container
4. The Version 2 open items are *listed* (not solved)
5. `TP-M5` is signed

When those hold, the feasibility-analysis verdict is confirmed **by evidence,
not by analysis** — that sentence is the POC's closing line, and these
scripts are how you get to it.
