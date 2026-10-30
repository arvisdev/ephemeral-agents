# M2 Delivery Plan — Container Lifecycle Manager

| | |
|---|---|
| **Milestone** | M2 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | The custom core: a Temporal activity that spawns an ephemeral executor container with per-run config/credentials and **reliably tears it down** on success, failure, and timeout. |
| **Depends on** | M0 (Docker on VM-Xb), M1 (workflow start path) |
| **Retires risk** | R1 (lifecycle leaks / crash recovery) — the largest custom risk in the POC |

## Design (decide and record before building)

- **Where it runs:** a Temporal **worker** + a **lifecycle service** on VM-Xb. The worker drives the durable workflow; the lifecycle service owns the Docker daemon (socket access) and exposes a small localhost API (`spawn`, `status`, `reap`) so the worker never holds a Docker client across retries.
- **State machine:** `creating → running → (complete | failed | timed-out) → reaped`. Persist state per `task_id` (a small table or labels/annotations on the container) so a crashed worker can resume or clean up.
- **Per-run config shape:** image tag (pinned), agent_ref + repo folder (from M1 message), behavioral spec, secrets reference (by name, not value — resolved at spawn), resource limits, TTL (**owner: `ttl_s` from the M1 message when present; otherwise the lifecycle config default `POC_TTL_DEFAULT` — the reaper is authoritative either way**).

## Work packages

**WP-2.1 — Image strategy**
- Define the two executor base images (placeholders in M2; real contents come in M3): a tagged, pre-pulled image per agent type.
- Images are **pre-pulled at deploy time** (compose `pull: always` + a deploy check); activities never pull. Record image tags in the deploy config.

**WP-2.2 — Spawn activity**
- Temporal activity that calls the lifecycle service `spawn`: creates the container with pinned image, env (config, not secrets), volumes (a per-task work dir), labels (`task_id`, owner=POC), resource limits, and a TTL.
- Timeouts: activity-level deadline + healthcheck poll; on timeout → graceful stop → force kill.

**WP-2.3 — Teardown**
- Idempotent `reap`: remove container + its per-task volume; safe to call multiple times; runs on **success, failure, and timeout**.
- Reaper backstop: a periodic job (cron sidecar or Temporal scheduled workflow) that reaps any POC-owned container past TTL or whose owning workflow is gone/unknown.

**WP-2.4 — Crash-recovery proof**
- `kill -9` the worker mid-activity → container must not be orphaned beyond TTL: reaper reaps it; the workflow retries the activity; final state is clean.
- 10 mixed runs (5 success, 3 injected failure, 2 timeout) → **zero** surviving POC containers afterwards.

**WP-2.5 — Observability**
- Structured per-run log (spawn/complete/reap events with `task_id`), container log capture to a local file per task, and a one-page status report (running tasks, last reap results). No dashboard required for POC.

## Deliverables

1. Lifecycle service + worker (POC repo, `services/lifecycle/`).
2. Pinned placeholder executor images + deploy config (pre-pull check).
3. Per-run log + reaper reports for the §WP-2.4 runs.
4. Short design note recording the state machine + service boundary (2–3 paragraphs).

## Acceptance criteria (done-when)

1. **Clean path:** spawn → stub hydrate → success → reaped; no artifacts left.
2. **Failure path:** injected failing + hung containers are force-reaped; workflow surfaces the error.
3. **Crash path:** killed worker + reaper ⇒ no orphaned POC containers.
4. **Scale check:** 10 mixed runs ⇒ zero survivors; teardown idempotent (double-reap is a no-op).
5. **No-pull rule:** activity time does not include an image pull (verified in logs).

## Exit gate

M3 may start when criteria 1–4 pass. If the reaper cannot be made crash-safe inside M2, **stop**: R1 is the POC's load-bearing risk and the whole point of sequencing M2 before the real executors.

## Notes

- This milestone intentionally uses **stub** executor containers (sleep/echo with a config echo-back) — the real Hermes/Claude Code images are M3's job. M2 proves *lifecycle*, not *agent*.
- The "Executes Activity" diamond in the diagram is exactly this workflow: one activity, two spawned containers (Hermes ephemeral + Claude Code (N+1)). M2 handles **one container per task** first; the N+1 fan-out (second container) is added in M3/M5 per the owner's concurrency decision (R3).
