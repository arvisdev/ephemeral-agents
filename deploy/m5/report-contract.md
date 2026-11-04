# M5 Report Contract — the human-report endpoint (engineer's desktop ↔ gateway)
<!-- Source: version 1/installation-deployment/M5-End-to-End-Installation-and-Deployment.md section 2/section 4 -->

The engineer's desktop talks **only to the gateway** (`http://vm-xa:8080`, LAN) —
the diagram's human-initiated, human-reported scope. This file pins the exact
endpoint contract so the human path is testable (E1), not hand-waved.

## Request (desktop → gateway)

`POST /` — the single JSON the gateway accepts:

```json
{ "title": "add health endpoint", "agent_capability": "dev", "task_brief": "..." }
```

Gateway response (immediate):

```json
{ "task_id": "<uuid>", "status": "accepted" }
```

The manager composes the M1 task message from the request (`{task_id,
agent_ref, behavioral_spec, repo_target, requester, ttl_s}` → topic
`agent.tasks`; `ttl_s = max(3 × B1 boot-time p50, estimate)`, manager sole writer).

## Report (desktop → gateway → manager → `/reports/`)

`GET /gateway/reports/<task_id>` — the gateway proxies the manager's
`/reports/<task_id>.json` (manager volume `/opt/poc/reports:/reports`).

```json
{
  "task_id": "<uuid>",
  "status": "success | failed | timeout",
  "outcome": "one-line human-readable summary",
  "artifact": { "branch": "task/<task_id>", "commit_sha": "<sha>" },
  "files_changed": ["..."],
  "error": "reason — present when status != success"
}
```

Polling contract: the desktop polls `GET /gateway/reports/<task_id>` until the
report exists (HTTP 200 with `status` set); 404 = still running. The report
file is written by the manager when it consumes the task's message from
`agent.tasks.response` (filtered by `task_id`) — the closing half of the
two-way Manager ↔ RabbitMQ path.
