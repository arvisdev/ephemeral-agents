# M1 Delivery Plan — Message-to-Workflow Bridge

| | |
|---|---|
| **Milestone** | M1 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | A RabbitMQ consumer that starts exactly one Temporal workflow per message — idempotent, at-least-once, no loss. |
| **Depends on** | M0 (RabbitMQ topic + Temporal server up and reachable) |
| **Retires risk** | R2 (bridge correctness / loss) |

**Why the broker exists (owner decision 2026-10-29):** Temporal alone could queue
these requests (its task queues are durable and the manager could start workflows
directly via the SDK). RabbitMQ is kept anyway because it is the **V2 ingestion
boundary**: other, non-Temporal systems will publish tasks into the topic alongside
the manager, and a broker is the right seam for multi-publisher fan-out. Everything
in this plan (manual ack, ack-after-start, idempotency, DLQ, no-loss tests) is the
price of owning that boundary — M1's job is to prove the price is manageable.

## Work packages

**WP-1.1 — Define the v1 message schema**
- One JSON schema for the task message published to the topic. Minimum fields:
  - `task_id` (unique, used as the idempotency key / Temporal workflow ID)
  - `agent_ref` — repo location + repo folder containing the agent's markdown (persona, skill and tools) — per the diagram's Note 1 ("the agent will pass the repo location, and repo folder that contains all the markdowns")
  - `behavioral_spec` — the Behavioral Specification passed by the manager (Note 1)
  - `repo_target` — Project Repo to write code to
  - `requester` — who asked; carries the response path. There is no separate `correlation_id`: the two-way Manager ↔ RabbitMQ response correlates on `task_id` (the single identifier for request and response).
- One optional field: `ttl_s` (integer seconds) — the manager is the only writer of it; when absent, M2's lifecycle applies its config default (`POC_TTL_DEFAULT`). The reaper stays authoritative regardless (R1).
- Deliverable: `schemas/message-v1.json` + one example message + a short schema-rationale note (including the `ttl_s` ownership rule).

**WP-1.2 — Build the adapter service**
- A durable RabbitMQ consumer (manual ack, prefetch = 1 for POC simplicity) that, per message:
  1. validates the message against the schema (reject + DLQ on invalid),
  2. starts a Temporal workflow with **workflow ID = `task_id`** (`WorkflowIDReusePolicy` + catch the "already exists" error → treat as success),
  3. acks **only after** the workflow start is confirmed.
- The adapter runs as a long-lived container on VM-Xb (it orchestrates VM-Xb resources). One compose entry + healthcheck.

**WP-1.3 — Idempotency & redelivery**
- Duplicate `task_id` → no second workflow (Temporal "workflow already exists" is the source of truth; no extra store needed for POC).
- Unacked-message redelivery after consumer crash must not double-start.
- Poison-pill handling: after N validation failures, route to a dead-letter queue; record the reason.

**WP-1.4 — Loss & crash testing**
- Publish N distinct messages → N distinct workflows (assert via Temporal visibility / `workflow list`).
- Kill the adapter mid-processing (SIGKILL) → after restart, no lost messages; in-flight message redelivered without a second workflow.
- Restart Temporal (server container) → adapter recovers and resumes consuming.

## Deliverables

1. Adapter source **in Go** (Temporal Go SDK — **mandatory**: owner instruction that all Temporal code be Go, for speed + size) in the POC repo under `services/topic-bridge/`.
2. Message schema + example (`docs/` or `schemas/`).
3. Test report: the §WP-1.4 scenarios with actual results.
4. Compose/healthcheck entries for VM-Xb.

## Acceptance criteria (done-when)

1. **N → N**: N published messages start exactly N workflows.
2. **Duplicates safe**: redelivering the same message starts no second workflow.
3. **No loss**: kill -9 of the adapter loses no messages (verified by count).
4. **Poison-safe**: invalid messages land in the DLQ with reason, never crash the consumer.
5. **Recovers**: Temporal or adapter restart resumes cleanly.

## Exit gate

M2 may start when criteria 1–5 pass against the M0 chassis. If idempotency by workflow ID proves awkward with the chosen Temporal SDK version, the fallback is a tiny Postgres idempotency table (same M1, +1 work package) — do **not** defer to M2.

## Notes

- Keep the adapter **dumb**: it translates topic → workflow start only. All agent logic lives in the workflow (M2+). This keeps M1 small and independently testable.
- The two-way Manager ↔ RabbitMQ response path (owner-confirmed) is modeled here as `requester`/correlation data on the message; the response channel itself is a separate queue introduced when M5 wires the human report — M1 only needs the correlation fields to exist.
