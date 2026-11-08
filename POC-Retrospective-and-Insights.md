# POC Retrospective & Insights — Ephemeral Agents

**Date:** 2026-11-07 · **Basis:** full POC record — 104 commits, 23 M4/M5 install runs,
per-milestone TP results with root-cause ledgers, M0/M5 decision logs, and the signed
Initial POC Findings Report. Companion to `Initial-POC-Findings-Report.md` (what we
built and measured); this document is **what we learned**.

---

## 1. Where the effort actually went

| Phase | Duration | Install/verify runs | Root causes fixed |
|---|---|---|---|
| Design → M0 | ~Oct 25 – Nov 5 | few | — |
| M1 (topic bridge) | ~1 day | few | ~2 |
| M2 (durable lifecycle) | ~1 day | 10 | **14** |
| M3 (executors + GitHub App) | ~1.5 days | 11 (TC-M3-04 saga) | ~9 |
| M4 (hindsight) | ~1 day | 6 | ~5 |
| M5 (front-of-house + E2E) | ~1 day | **17** | ~8 |
| **Total** | **~4 days of build** | **~44 runs** | **~35–38** |

**Insight 1 — Defects cluster at integration boundaries, not in components.**
Every service worked in isolation almost immediately; nearly all of the 35+ root
causes are boundary defects: spawn-env contract mismatches (`TASK_PROMPT` unbound,
`TTL` vs `TTL_S`), name/shape mismatches (container names, index.json shapes,
queue names, `REPO_URL` construction), and deployment-context mismatches (probing
host ports for compose-network-only services, PATH in non-interactive ssh,
`/tmp`-resident proxy configs). **V2 guidance: invest in contract tests at the
boundaries (schema fixtures + spawn-env integration tests) before milestone
gates, not during them.** Each boundary bug cost a full install re-run (~10 min)
to discover because gates run end-to-end.

**Insight 2 — The "gate asserts the outcome" rule is the single most important
process fix.** Run 16 "passed" §4 while the acceptance task had actually failed:
the gate checked reaped cleanliness (absence of debris) instead of the report's
`status`. Once the gate asserted *task success*, three latent bugs surfaced
immediately (set-u abort, stale proxy, executor routing). **A verification that
only proves cleanup happened is not a verification of the work.** V2: every gate
assertion should name the artifact it proves, not the absence of failure.

**Insight 3 — Harness-verified ≠ integration-verified.** M4's gate tests spawned
executor containers directly with full env and passed; the first real M2-lifecycle
spawn exposed the `TASK_PROMPT` abort within seconds. The POC's own milestone
structure (each milestone re-running the previous milestone's real path) is what
caught it — that cross-milestone regression discipline earned its cost.

**Insight 4 — Ephemeral configuration is a correctness hazard.** The llm-proxy
sanitize fix lived only in `/tmp/proxy.js` on the VM and silently regressed on
redeploy, resurfacing a bug we had already fixed and forgotten. The fix cost
minutes; the diagnosis cost an hour. **V2 rule: every runtime artifact must be
canonical in the repo — nothing load-bearing lives only on a VM.** (Also true of
the executors themselves: registry-only image sourcing is what made re-runs
deterministic at all.)

**Insight 5 — Pins are load-bearing and must be audited, not just recorded.**
Pinning (Claude Code 2.1.318, Hermes v0.23.0, registry tags) gave reproducibility,
but both pinned CLIs bundle vulnerable dependency trees that we cannot patch —
the only remediation is upstream. The §6 exception process (recorded residuals
with rationale) kept the gate honest without lying about risk. **V2: pin + track
upstream release feeds for the pinned artifacts; the exception list should have
expiry dates.**

## 2. What the architecture proved (and didn't)

**Proven with evidence:**
- **Temporal durability is the right spine.** Every crash, restart, and mid-task
  manager kill (F5) recovered without manual state surgery; the workflow result
  mapping (`complete|failed|timed-out`) never lost a task. The decision to write
  *all* orchestration in Go (D-8/owner) held up — zero language-boundary defects.
- **Ephemeral single-task containers with label-driven reaping** delivered a
  clean-slate guarantee we could actually *gate on*: `survivors=0` after every run,
  across ~40 runs. This is the POC's most reusable operational idea.
- **Scoped, revocable credentials work end-to-end.** GitHub App → 540 s JWT →
  per-task `ghs_` token → verified 401 within ~75 s of reaping. No long-lived
  secret ever entered a container. This is the security architecture's strongest
  result and should be carried into V2 unchanged.
- **Capability routing** (`agent_capability` → index.json → agent_ref → executor)
  resolved correctly from a human's one-line request; the manager composing the
  work package (spec + brief) is the right place for that logic.
- **Cross-task memory hydrates fast and cheap** (13 ms read; learnings visibly
  injected into later tasks' boot logs). The write-on-end contract worked with
  zero Go/Temporal changes (bash-only), which kept the blast radius small.
- **Latency is not a barrier:** p50 boot-to-ready 785 ms; full acceptance ~3 min;
  coder spawn-to-commit ~3 min. The system's overhead is small relative to agent
  work time.

**Not proven / deliberately deferred (the honest gaps):**
- **Concurrency beyond one-coder-per-task** — N-parallel + worktrees untested.
- **Isolation strength** — co-located containers share the VM; a malicious/failed
  agent's blast radius is the VM, not a VM-per-task.
- **Semantic memory** — EMBEDDINGS=none means cross-task "memory" is exact-record
  recall, not retrieval. The hydration pipeline is ready; the provider isn't.
- **Scale** — one task at a time on one VM; queue depth, multi-VM scheduling, and
  back-pressure untested.
- **The hermes executor's "real runtime"** — its exit artifact is the hydration
  chain + report; the coder does the real repository work. If V2 wants the Hermes
  harness itself doing autonomous multi-tool work in-loop, that integration is
  still to be built (it was the owner's packaging boundary in the POC).

## 3. Method-level insights (what made 4 days possible)

1. **Rerun-material convention (all test inputs in the repo)** was the highest-
   leverage rule. It made every gate idempotent and every re-run byte-comparable.
   Violations of it (the /tmp proxy) were the only nondeterminism we ever hit.
2. **Root-cause-only fixes, committed with rationale** turned the defect list into
   a design document. The tp-m2 ledger (14 causes) reads as a spec of the lifecycle's
   real semantics — better than any upfront design would have been.
3. **Standing authorization + batched approvals** kept the loop tight; the owner's
   involvement condensed to: GitHub App creation (unavoidable — no API), IP
   reassignment, and vm-hindsight re-provisioning. Everything else was agent-driven.
4. **Stage-gated installs (S1–S7)** in M5 localized failures to a single stage,
   which is why 17 runs took ~1 day instead of days. Earlier milestones paid this
   cost repeatedly; M5's structure should be the default from M0 in V2.
5. **Verbatim-doc discipline (don't "fix" the design docs)** prevented the common
   failure of retro-fitting reality into the spec; instead reality was recorded in
   decision logs and the docs stay the owner's to amend. The topology-diagram
   deferral is the exemplar.

## 4. Security posture findings

- The **App-token lifecycle is provably short** (≤1 h cap, ≤75 s observed revocation
  lag) and scoped to two repos with Contents-only rights — the minimum viable blast
  radius. Model: extend to per-agent scopes in V2 (today the App install is
  account-wide to two repos).
- **Secret-scan gates (secrets=0) caught real issues** (tokens in evidence files,
  image layers) and are cheap. Keep as a hard gate.
- The residual risk register is small but real: base-image OS CVEs (unfixable
  upstream), bundled CLI deps, and the plain-HTTP registry (insecure-registries)
  — acceptable on a LAN POC, **not** acceptable in any multi-tenant V2.
- The estate-wide **sudoers grant** (`99-arvisdev`, passwordless) and the
  workstation-held `gh` keyring token are the two credentials that would hurt most
  if the workstation were compromised. V2: scope both, rotate, and expire.

## 5. Failure-mode taxonomy (for the V2 runbook)

Across all milestones, every failure fell into one of five buckets:

1. **Contract mismatch** (env var names/shapes, message fields) — ~40% of defects.
2. **Deployment-context mismatch** (network reachability, PATH, mounts, /tmp) — ~25%.
3. **Timing/race** (poll windows, TTL grace, queue-drain races) — ~15%.
4. **Third-party API semantics** (OpenRouter 422, GitHub JWT caps, permission modes) — ~15%.
5. **Estate drift** (deleted/replaced VMs, IP reassignment) — ~5%, but highest severity.

**V2 guidance:** contract tests (1), hermetic deploy contexts (2), explicit timing
budgets in gates (3), a pinned-and-updated adapter layer for third parties (4),
and a documented estate inventory with drift detection (5).

## 6. Recommended V2 priorities (derived, in order)

1. **Contract/integration test suite** at every boundary (spawn-env, message-v1,
   catalog shapes) — would have prevented ~half the run count.
2. **Canonical-asset rule enforcement** (CI check: no runtime config sourced from
   outside the repo).
3. **Concurrency model** (N-parallel + worktrees) — the first real capability gap.
4. **Isolation upgrade path** (per-task VM/microVM) — the first real security gap.
5. **Embeddings provider decision** — unlocks the memory system's design intent.
6. **Upstream pin refresh pipeline** (Claude Code, Hermes) with exception expiry.
7. **Registry TLS + credential rotation** for anything beyond the LAN.
8. **Re-draw ARCHITECTURE.md / foundation topology** (owner) to match the proven
   4-VM→5-service estate.

## 7. Open questions for the owner

- Which insight #6 priority order matches your intent for V2?
- Should the V2 baseline reuse the POC estate as-is (grow-in-place) or start from a
  fresh provisioning (the M0 pattern is now a ~30-minute script)?
- Is the "hermes executor as the in-loop autonomous agent" integration in scope for
  V2, or does V2 keep the coder (real-edit) executor as the primary workhorse?
