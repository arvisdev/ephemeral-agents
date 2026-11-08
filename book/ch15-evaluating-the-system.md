# Chapter 15 — Evaluating the System: The Evaluator's Guide

> **In plain terms:** Everything before this chapter described the system. This chapter is for
> the person who wants to prove those descriptions themselves — to submit a real request,
> watch it travel through the machinery, and verify each claim against the running system
> rather than the book's word. Throughout this chapter, `<gateway-host>` is the front-of-house
> VM, `<execution-host>` is the execution VM, and `<user>` is the deployment account. All
> concrete values (addresses, ports, credentials) live with the operator in the deployment
> environment files (`hosts.env`, `deploy/m0/.env`) and are deliberately not printed here.

## Read this first — POC boundary

**This system is a proof-of-concept. It is not production-ready, and it must not be operated
as such.** The POC proves the architecture; it does not yet provide the hardening of a
product. An evaluator must understand precisely where that boundary lies before touching
anything:

- **No authentication on the front door.** The gateway (`http://<gateway-host>:8080`) and the
  Temporal web UI (`http://<execution-host>:8080`) are reachable by any host on the local
  network without credentials. Any network-attached user can submit tasks and read any
  workflow's history.
- **The perimeter is the network, nothing more.** Credentials never live in VMs or containers
  (the least-privilege design of Chapter 8 holds — every executor token is minted per task and
  revoked after), but the *interfaces* themselves are unauthenticated and the deployment is
  intended for an isolated evaluation network only.
- **Single-host execution.** One execution VM carries the entire worker side. There is no
  horizontal scaling, no multi-tenancy, no rate limiting, and isolation is container-level,
  not host-level.
- **POC-grade evidence, not a product.** The signed gate results (Chapters 11–12) are the
  record of one build on one estate under one operator. Production hardening —
  authentication, stronger isolation, scale-out, the conversational front door — is scoped in
  Chapter 13 and remains deliberately open.

**If your evaluation environment shares a network with anything you cannot risk exposing,
stop and isolate it first.**

## Prerequisites

1. A running deployment, either provisioned from scratch (Chapter 14) or by the milestone
   install scripts under `deploy/`. The evaluator needs SSH access to both the front-of-house
   VM (`<gateway-host>`) and the execution VM (`<execution-host>`), as `<user>`.
2. The agent catalog repository reachable, with at least one agent entry in its
   `index.json` (the manager resolves every request against this catalog).
3. The request-format fields (from `schemas/message-v1.json`), restated for
   convenience below.

> **If the estate was recently rebooted,** bring the stack back in
> dependency order first (Chapter 14 §14.8) — a reboot does not restore the
> deployment in the right order, and components that resolve the workflow
> engine by name will crash-loop until it exists.

| Field | Meaning |
|---|---|
| `title` | A short human label for the task (appears in the report) |
| `agent_capability` | The capability keyword the manager matches against the catalog |
| `task_brief` | The actual instruction given to the ephemeral agent |

## The evaluation flow

### 1. Health check

```bash
curl http://<gateway-host>:8080/healthz
```

Expect an `ok`-style 200 response. If this fails, the deployment is not up; return to the
install stage that owns the gateway.

### 2. Submit a request

```bash
curl -s -X POST http://<gateway-host>:8080/ \
  -H 'Content-Type: application/json' \
  -d '{"title": "Evaluator smoke test", "agent_capability": "<capability-from-catalog>", \
       "task_brief": "Create a file named hello.txt containing the word hello."}'
```

The gateway replies **immediately** with:

```json
{"task_id": "<uuid>", "agent_ref": "<resolved agent reference>", "ttl_s": 540}
```

That the response is instant matters: the gateway holds no state and no credentials — it
mints a task id, validates the shape, and hands off to the manager. The TTL (540 s) is the
task's lifetime budget; the executor must finish inside it or the task is retired.

> **Capability note:** the manager routes by exact match, then case-insensitive substring,
> against the catalog's `agent_capability` values. An evaluator should pick a capability
> that genuinely appears in `index.json` — a fuzzy request may still route to the nearest
> agent (this is the documented keyword-matching limitation, not a bug).

### 3. Poll the report

```bash
curl -s http://<gateway-host>:8080/gateway/reports/<task_id>
```

Poll until the report appears. **Success** is `"status": "success"` with an artifact pointer
(a commit or file reference). **Failure** carries a human-readable `reason` — read it, and
see the failure section below.

### 4. Verify zero survivors

On the execution VM, after the task completes:

```bash
ssh <user>@<execution-host> 'docker ps --filter "name=poc-" --format "{{.Names}}"'
```

Expect an empty list. The zero-survivors guarantee (Chapter 6) means nothing of the agent
persists beyond its artifacts and its memory entry. This check is how you verify it yourself,
rather than taking the book's word.

## Dashboards & observability — watch the machinery yourself

The gateway report tells you what the system *says* happened. The two dashboards on the
execution VM let you watch what *actually* happened, in the infrastructure itself. That is
the difference between trusting a report and auditing it.

### Temporal web UI — the workflow view

Open `http://<execution-host>:8080` in a browser (anywhere on the evaluation network).

1. In the workflow list, locate the workflow whose ID is your `task_id`. **Exactly one**
   `TaskWorkflow` should exist per submitted request — verify this by eye (claim TC-M5-01).
2. Open it and read the **event history**: workflow started → activity scheduled → activity
   completed → workflow completed. This timeline is the durable-engine claim of Chapter 5
   made visible — the work was coordinated by a workflow engine, not a process.
3. The **input** shows the work package the manager composed; the **result** shows the
   executor's outcome payload. An evaluator can read both without touching the code.
4. On a failed task, the history shows the retry chain and the failing event — the same
   view the maintainers used in the failure ledger (Chapter 12).

> **Boundary reminder:** this UI is unauthenticated and LAN-open — inherited from the POC
> boundary above. Do not expose it beyond the evaluation network.

### RabbitMQ management — the queue view

The management plane is deliberately bound to loopback only on the execution VM (the M0
security design). Reach it through an SSH tunnel from your evaluation workstation:

```bash
ssh -N -L 15672:127.0.0.1:15672 <user>@<execution-host>
# then open http://localhost:15672 in a browser
```

Sign in as the deployment's message-broker user (operator provides the credentials from
`deploy/m0/.env` — they are intentionally not recorded in this book).

1. **Queues → `agent.tasks.durable`:** submit an evaluation request and watch the message
   land (depth 0→1) and drain (1→0) as the worker consumes it. The publish/consume rate
   charts show the same thing live.
2. **Exchanges → the topic exchange:** confirm the task message routed to the durable queue
   (the D-routing claim of Chapter 7).
3. **Consumers:** exactly one consumer on the durable queue while the deployment is healthy.

> **Boundary reminder:** the loopback-only binding is a deliberate protection, not an
> oversight. If you find the management plane reachable without a tunnel, stop and tell the
> operator — that would be a regression against the M0 design.

### The live evaluation checklist

For the fullest single-run evaluation, run these in order while one request executes:

| When | Where | What you see | Which claim it verifies |
|---|---|---|---|
| t=0 | gateway | Instant `{task_id, agent_ref, ttl_s}` response | Stateless, credential-free front door (Ch. 8) |
| t≈1 s | RabbitMQ | Message lands in `agent.tasks.durable`, then drains | Manager published; worker consumes (Ch. 7) |
| t≈2 s | Temporal UI | `TaskWorkflow` appears for the task_id | Exactly one workflow per request; durable engine (Ch. 5) |
| t≈3–4 s | Temporal UI | Credential-mint activity completes | Scoped, per-task token (Ch. 8) |
| during | execution VM | `poc-<task_id>` container running | Ephemeral execution (Ch. 6) |
| at end | execution VM | Container gone; queue empty | Zero survivors (Ch. 6) |
| at end | gateway | Report `status: success` | Credentialed write-back to the target repo (Ch. 8) |

If every row checks out, you have independently reproduced the POC's end-to-end claim —
not from this book's testimony, but from your own observations of the running system.

## Evaluating failure behavior

The POC's failure story is part of its claim. An evaluator can exercise it:

- **Read the reason.** A failed report carries a human-readable `reason` string, and the
  Temporal history shows the exact failing event. Failures are supposed to be *legible* —
  this is the Chapter 12 lesson: gates must assert outcomes, not just cleanliness.
- **Reproduce an injection.** The five failure-injection classes run during the build
  (§5 of the install: unreachable broker, workflow failure, executor crash, token
  revocation, reap cleanliness) are re-runnable by an evaluator with operator help: stop
  RabbitMQ, kill the executor mid-task, or let a task exceed its TTL, and confirm the system
  fails *safely* — the workflow retries or the task is retired with a clean report, tokens
  revoked, containers reaped.
- **Inspect the accepted residuals.** The security scan story (§6) is honest about what is
  *not* fixed: language-package findings inside pinned third-party runtimes are recorded as
  accepted residuals with per-image evidence. An evaluator should read those exception
  records in the milestone decision logs and judge whether the residual list is honest.

## Claim-by-claim verification

The table below maps the POC's headline claims (from the findings report) to the
evaluator-observable checks in this chapter, so each claim can be confirmed hands-on:

| POC claim | Verify by |
|---|---|
| A human request returns a task id in ~1 second | The instant gateway response (§2) |
| Tasks survive machine failure (durable core) | Temporal workflow timeline survives a container restart (§3) |
| Executors are ephemeral — nothing persists | Zero-survivor check (§4) + reap observation |
| Every executor token is least-privilege and revoked | Mint activity in Temporal history; revocation within the task's lifetime (§3) |
| Exactly one workflow per accepted request | Temporal UI count (§3) |
| The queue is the execution boundary, validated before publish | Manager validation in Temporal input + queue drain view (§3) |
| Failures produce legible reports | Deliberate-injection run (§5) |

## The front door: today and next

**Today (Mode A):** the structured HTTP POST documented above is the *only* wired entry
point. A human — or their assistant — composes the request and submits it.

**Next (Mode B):** the designed V2 front door places a persistent Chief-of-Staff agent
between conversation and the gateway (Chapter 10): a natural-language request is decomposed
into work packages, agents are selected from the roster, and each package is submitted
through the very same gateway API this chapter documents. Every command an evaluator runs
here is the manual equivalent of what the CoS will do — the machinery being evaluated is the
machinery V2 will build on, unchanged.

---

*All host references in this chapter are placeholders by convention (see the book's
preamble). Concrete addresses, ports, and credentials live with the operator in the
deployment environment files and are never printed in this book.*
