# M2 Installation & Deployment — Container Lifecycle Manager

**Companion:** `../delivery plans/M2-Container-Lifecycle-Manager-Delivery-Plan.md` · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | vm-xb (lifecycle service + Temporal worker + reaper) |
| **Prereqs** | M0 (Docker + Temporal on vm-xb), M1 (workflows startable); a stub executor image built (§1) |
| **What gets deployed** | `poc/lifecycle:1.0.0` (API + reaper), `poc/worker:1.0.0` (Temporal worker running the activity), `poc/executor-stub:1.0.0` (placeholder executor) |

## 1. Build the pieces

All under `services/lifecycle/` in the POC repo.

**1a. Stub executor image** — `deploy/m2/executor-stub/Dockerfile`:

```dockerfile
FROM debian:trixie-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
```

`entrypoint.sh`: echo back the config env (`TASK_ID`, `AGENT_REF`, `BEHAVIORAL_SPEC`) to stdout, create a marker file, then `sleep $TTL` (responds to SIGTERM). This proves lifecycle mechanics with **zero agent logic** — M3 replaces it with real images.

**1b. Lifecycle service** (Go) — HTTP API on loopback only (`127.0.0.1:8090`):

- `POST /spawn`  — `docker run -d` the pinned image: env config, per-task work volume, labels `poc.task=<task_id>`, `poc.owner=lifecycle`, `poc.expires=<RFC3339 TTL>`, resource limits; returns container ID.
- `GET  /status?task=<id>` — container state + health.
- `POST /reap?task=<id>` — idempotent: `docker rm -f` + remove per-task volume; no-ops when absent.
- **Reaper loop** (every 30 s in the same process): `docker ps -aq --filter label=poc.owner=lifecycle` → reap any container past `poc.expires` or whose owning workflow is no longer running (queried via Temporal client).
- Docker access: mount `/var/run/docker.sock` (no docker-in-docker).
- State: container labels + a JSON state file on a per-task volume; no DB for POC.

**1c. Worker** (Go) — Temporal worker registered on `default` namespace; activity `executes_activity(taskCfg)`:
`spawn → poll status until complete/failed/TTL → reap → return result`. Activity retry policy: `MaximumAttempts=2`, start-to-close timeout from task config; workflow-side timeout backstop so a lost activity still reaps (reaper is the real backstop).

```bash
docker build -f deploy/m2/Dockerfile.services -t poc/lifecycle:1.0.0 -t poc/worker:1.0.0 services/lifecycle
docker build -f deploy/m2/executor-stub/Dockerfile -t poc/executor-stub:1.0.0 deploy/m2/executor-stub
for img in poc/lifecycle:1.0.0 poc/worker:1.0.0 poc/executor-stub:1.0.0; do
  docker save $img | ssh vm-xb 'docker load'
done
```

## 2. Deploy on vm-xb — `deploy/m2/compose.yaml`

```yaml
services:
  lifecycle:
    image: poc/lifecycle:1.0.0
    environment:
      DOCKER_HOST: unix:///var/run/docker.sock
      LISTEN: 127.0.0.1:8090
      REAPER_INTERVAL: 30s
      POC_IMAGE: poc/executor-stub:1.0.0     # pinned — no pulls at activity time
      POC_PULL_AT_DEPLOY: "true"
    volumes: ["/var/run/docker.sock:/var/run/docker.sock:ro"]
    restart: unless-stopped
    # deploy gate (deploy/m2/deploy.sh): the service entrypoint RUNS `docker image inspect poc/executor-stub:1.0.0`
    # on boot and refuses to serve until the pinned image exists locally (the no-pull rule, enforced).
  worker:
    image: poc/worker:1.0.0
    environment:
      TEMPORAL_ADDR: temporal:7233
      LIFECYCLE_API: http://lifecycle:8090     # same compose project; expose the API to this network
    depends_on: [lifecycle]
    restart: unless-stopped
```

Pre-deploy gate:

```bash
ssh vm-xb 'docker compose -f /opt/poc/deploy/m2/compose.yaml exec lifecycle \
  test -n "$(curl -s http://127.0.0.1:8090/health)"'
```

## 3. Run the verification suite (the M2 gate tests)

Driver: `deploy/m2/tests/run-task.sh '<taskJson>'` → starts a `TaskWorkflow` via Temporal CLI and waits for completion.

```bash
# V1 clean path
./tests/run-task.sh '{"task_id":"t001","ttl_s":30}'        # expect: success; docker ps -a | grep poc.task=t001 → gone
# V2 failure path
./tests/run-task.sh '{"task_id":"t002","inject":"exit-1"}' # expect: workflow reports failure; container reaped
# V3 timeout path
./tests/run-task.sh '{"task_id":"t003","inject":"hang","ttl_s":15}'  # expect: reaped at TTL; error surfaced
# V4 crash path  (the load-bearing test)
./tests/run-task.sh '{"task_id":"t004","inject":"hang","ttl_s":120} &' sleep 5
docker kill worker                       # worker dies mid-activity
sleep 35                                 # reaper interval on `lifecycle` keeps running
docker ps -a --filter label=poc.task=t004                          # expect: none — reaper got it
docker start worker                     # workflow retries; final state clean
# V5 10 mixed runs, zero survivors
./tests/mixed-runs.sh 10 && docker ps -a --filter label=poc.owner=lifecycle -q | wc -l    # expect 0
```

Record results in `deploy/m2/evidence/m2-tests.txt`.

## 4. Observability (M2.5, same deploy)

- Structured JSON logs: one line per lifecycle event `{ts, task_id, event: spawn|complete|fail|timeout|reap, container_id}`.
- Per-task container logs captured to `/var/poc/tasks/<task_id>.log` (volume), retained until reaped + 24 h.
- `GET /report` → one-page status (running tasks, last 10 reap events, reaper health). No dashboard for POC.

## 5. Rollback / re-do

```bash
cd deploy/m2 && docker compose down          # worker + lifecycle down
docker ps -a --filter label=poc.owner=lifecycle | xargs -r docker rm -f   # manual reap of anything left
# no persistent data: state lived in labels + /var/poc/tasks (wipe /var/poc on full redo)
```

## 6. Troubleshooting

| Symptom | Cause / fix |
|---|---|
| `cannot connect to Docker daemon` | socket not mounted or `DOCKER_HOST` unset; check `deploy/m2/compose.yaml` volume line |
| Reaper reaps healthy containers | `poc.expires` label missing on spawn (label added only in 1b spawn — verify `docker inspect`), or clock skew between containers on vm-xb (`chrony`) |
| Activity stuck, no error | start-to-close timeout not set; check the workflow's activity options |
| `No such image` at spawn time | deploy gate bypassed — re-run `deploy/m2/deploy.sh` (the gate refuses to serve without the pinned image) |
| Container leaks after worker kill | reaper interval too long vs. TTL, or reaper query filter mismatched (`poc.owner=lifecycle` — exact match) |
