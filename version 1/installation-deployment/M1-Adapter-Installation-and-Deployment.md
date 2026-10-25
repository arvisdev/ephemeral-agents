# M1 Installation & Deployment — RabbitMQ → Temporal Adapter

**Companion:** `../delivery plans/M1-RabbitMQ-Temporal-Adapter-Delivery-Plan.md` · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | vm-xb (the adapter service container), built on the owner's workstation or a build VM |
| **Prereqs** | M0 gate passed: RabbitMQ topic `agent.tasks` + queue `agent.tasks.durable` up; Temporal reachable on vm-xb:7233 |
| **What gets deployed** | 1 container on vm-xb: `poc/topic-bridge:1.0.0` (+ its compose entry, env, healthcheck) |
| **Language (mandatory)** | The adapter **must be Go** (Temporal Go SDK) — owner instruction: all Temporal code in Go for speed + size. |

## 0. Prereq checks (run these first)

*Why:* if the queue or Temporal is down, the M1 gate tests produce *false* failures that look like adapter bugs. Two commands now save an hour of mis-debugging later — this is the junior-operator's first habit: verify the gate you're standing on before building on it.

```bash
ssh vm-xb 'curl -s -u poc:*** $(grep RABBITMQ_PASS deploy/m0/rabbitmq/.env | cut -d= -f2) \
  http://127.0.0.1:15672/api/queues/%2F/agent.tasks.durable | grep -o "\"messages\":[0-9]*"'
temporal operator cluster health
```

## 1. Build the adapter (`services/topic-bridge/` in the POC repo)

*Why:* the four choices in the correctness contract are the entire POC's at-least-once story: **prefetch=1 + manual ack** means one in-flight message; **`task_id` as `WorkflowID`** is the idempotency key that makes duplicates no-ops; **DLQ for invalid messages** means a poison pill is quarantined, never requeued into a crash loop. M2 and M5 assume every one of these — changing any of them later is a cross-milestone change.

Stack: **Go + Temporal Go SDK + amqp091-go** — **mandatory** (owner instruction: all Temporal code in Go, for speed + size; Temporal's multi-language support does not apply to this POC).

```bash
cd services/topic-bridge
go mod init poc/topic-bridge
go get github.com/temporalio/sdk-go@latest github.com/rabbitmq/amqp091-go@latest
```

The consumer must do, in order (this is the correctness contract — M2+ assumes it):

1. `QoS(prefetch=1)`, **manual ack**.
2. Consume `agent.tasks.durable` bound to topic exchange `agent.tasks`.
3. On message: validate against `schemas/message-v1.json` → **invalid: publish to `agent.tasks.dlq` with the reason as property, then ack** (never requeue invalid messages).
4. Valid: `client.StartWorkflow(ctx, StartWorkflowOptions{ WorkflowID: msg.TaskID, WorkflowIDReusePolicy: AllowDuplicate }, TaskWorkflow, msg)` → on `ErrWorkflowExecutionAlreadyExists` **ack** (idempotent duplicate) → otherwise **ack only after start confirmed**.

## 2. Package as an image — `deploy/m1/Dockerfile`

*Why:* the POC has no container registry — `docker save | ssh docker load` is the transfer path, and it's deliberate: the POC must run on a LAN with no external dependencies. The static multi-stage build keeps the consumer a **single Go binary** — the speed + size mandate made concrete.

```dockerfile
FROM golang:1.25 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/adapter ./cmd/adapter

FROM debian:trixie-slim
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/adapter /usr/local/bin/adapter
ENTRYPOINT ["adapter"]
```

```bash
docker build -f deploy/m1/Dockerfile services/topic-bridge -t poc/topic-bridge:1.0.0
docker save poc/topic-bridge:1.0.0 | ssh vm-xb 'docker load'    # POC transfer path, no registry
```

## 3. Deploy on vm-xb — `deploy/m1/compose.yaml`

*Why:* `depends_on: service_healthy` makes the bridge start only when RabbitMQ and Temporal are *actually* ready — no more "restart until it works". The shared external network is how the bridge reaches M0's services without re-deploying them; recording the network name keeps the next operator from guessing.

```yaml
services:
  topic-bridge:
    image: poc/topic-bridge:1.0.0
    environment:
      RABBITMQ_URL: amqp://poc:${RABBITMQ_PASS}@rabbitmq:5672     # same host network/compose project as M0's rabbitmq
      TEMPORAL_ADDR: temporal:7233
      TEMPORAL_NAMESPACE: default
      TASKS_EXCHANGE: agent.tasks
      TASKS_QUEUE: agent.tasks.durable
      DLQ_QUEUE: agent.tasks.dlq
    depends_on:
      rabbitmq: {condition: service_healthy}
      temporal: {condition: service_healthy}
    restart: unless-stopped
    logging: {driver: json-file, options: {max-size: "10m", max-file: "3"}}
```

(If M0's services live in a separate compose project on vm-xb, join them with a shared external network — record the network name in `image-tags.env`-style env notes.)

```bash
cd deploy/m1 && docker compose up -d topic-bridge
docker compose ps && docker logs -f topic-bridge        # expect: consumer started, connected, consuming
```

## 4. Verification run (the M1 gate tests)

*Why:* T1–T5 each corresponds to a failure mode the POC must survive later — duplicates (T2), a crash mid-flight (T3), poison messages (T4), engine restart (T5). They *are* the M1 gate: all five must pass before M2's installation begins, and the evidence file is what `TP-M1` signs.

Test harness: `deploy/m1/tests/publish.sh` + `assert.sh` (plain scripts, committed with the repo).

```bash
# T1: N -> N   (N=50)
./tests/publish.sh 50
sleep 5
temporal workflow list --type TaskWorkflow --limit 100 | grep -c running        # expect 50
# T2: duplicate safety — republish all 50
./tests/publish.sh 50 --dup
temporal workflow list --type TaskWorkflow | grep -c running                    # still 50
# T3: kill -9 mid-flight
./tests/publish.sh 20 & sleep 1; docker kill topic-bridge; docker start topic-bridge
sleep 5; temporal workflow list --type TaskWorkflow | grep -c running           # +20 new, no doubles
# T4: poison pill
./tests/publish.sh --invalid 3
rabbitmqadmin get queue=agent.tasks.dlq count                                  # expect 3; consumer still up
# T5: Temporal restart
docker restart temporal && sleep 30; ./tests/publish.sh 5                       # consumer recovers, 5 start
```

Record each result in `deploy/m1/evidence/m1-tests.txt`.

## 5. Rollback / re-do

*Why:* the adapter is stateless — rollback is one command — and the queue state is durable in RabbitMQ, so rolling back **never loses in-flight work**. This is the property that makes re-running M1 cheap at any point.

```bash
cd deploy/m1 && docker compose down topic-bridge      # stateless: no local data lost
# queue state (messages not yet consumed) is durable in RabbitMQ — nothing to recover
```

## 6. Troubleshooting

*Why:* these are the top failure modes, pre-diagnosed — each row maps a symptom (a log line or a count) to its cause, so a junior operator fixes in minutes instead of hours. The "duplicates starting workflows" row is the most important: it's the correctness contract (§1) observed from the outside.

| Symptom | Cause / fix |
|---|---|
| `connection refused` to `rabbitmq:5672` | adapter and rabbitmq not on the shared compose network (§3 note) |
| Adapter acks but no workflow | namespace mismatch (`TEMPORAL_NAMESPACE` vs `temporal operator namespace list`) |
| Duplicates starting workflows | `WorkflowID` not set to `task_id` — that is THE idempotency key; check the start options |
| Consumer stuck after crash | unacked message redelivered — expected; if it double-starts, T2's assertion is wrong before T3's fix |
| High memory | prefetch raised without cause; keep `prefetch=1` for POC scale |
