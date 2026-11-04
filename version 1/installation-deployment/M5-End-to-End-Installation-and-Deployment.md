# M5 Installation & Deployment — End-to-End Single Flow + Hardening

**Companion:** `../delivery plans/M5-End-to-End-Hardening-Delivery-Plan.md` · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | vm-xa (Gateway + Manager Agent), engineer's desktop (Hermes Desktop config) |
| **Prereqs** | M0–M4 gates all passed (chassis, adapter, lifecycle, executors, memory + knowledge) |
| **What gets deployed** | `poc/gateway:1.0.0` + `poc/manager:1.0.0` on vm-xa; the ordered full-stack bring-up; the failure-injection + hardening runbooks |
| **Language (mandatory)** | Every Temporal component in the stack (M1 adapter, M2 worker/lifecycle) is Go — owner instruction: all Temporal code in Go for speed + size. M5 adds no new Temporal code; the gateway/manager services are Go. |

M5 adds **no new infrastructure** — it composes what M0–M4 installed and locks the last open items.

## 1. Front-of-house on vm-xa — `deploy/m5/compose.yaml`

*Why:* the gateway + manager are the diagram's "tall narrow bar" made concrete, and Note 1's manager behavior (catalog search → agent artifact → task message → consume response) gets its concrete spec here for the first time. The manager container is **persistent by design** — it's the human-facing state that survives across tasks, the one deliberately non-ephemeral component.

```yaml
services:
  gateway:
    image: poc/gateway:1.0.0
    # Hermes Agent Gateway: tall narrow bar in the diagram — the request entry point
    ports: ["8080"]
    environment:
      LISTEN: 0.0.0.0:8080
      MANAGER_API: http://manager:8086
    depends_on: {manager: {condition: service_started}}
    restart: unless-stopped
  manager:
    image: poc/manager:1.0.0
    # persistent Hermes Agent (Docker Container) hosting the Manager Agent — survives across tasks
    environment:
      RABBITMQ_URL: amqp://poc:${RABBITMQ_PASS}@vm-xb:5672
      AGENT_LIST_REPO: git@github.com:<owner>/available-agent-list.git
      GITHUB_READ_TOKEN: ${AGENT_LIST_READ_TOKEN}
      TASKS_EXCHANGE: agent.tasks
      RESPONSE_QUEUE: agent.tasks.response
    volumes: ["/opt/poc/reports:/reports"]     # human-report artifacts land here
    restart: unless-stopped
```

**Manager behavior (Note 1, made concrete)** — the manager entrypoint, per request received from the gateway:

1. `git clone --depth 1` the Available Agent List (read token).
2. Search `index.json` for the agent matching the request (by role/capability in the request payload).
3. Locate the folder: full persona + skill and tools + the folder containing all the markdowns + the Behavioral Specification.
4. Compose the M1 task message: `{task_id, agent_ref, behavioral_spec, repo_target, requester, ttl_s}` → publish to `agent.tasks` (topic). **`ttl_s` is set by the manager (the only writer): `max(3 × B1 boot-time p50, brief estimate)`; absent ⇒ M2 applies `POC_TTL_DEFAULT` (900s). The reaper is authoritative regardless.**
5. Consume `agent.tasks.response` filtered by `task_id` → write the human report (outcome + artifact pointer: `task/<id>` branch / commit) to `/reports/<task_id>.json` → surface it to the engineer's desktop (the two-way Manager ↔ RabbitMQ path, owner-confirmed).

## 2. Engineer's desktop (external actor)

*Why:* the engineer's desktop talks **only to the gateway** — that is the diagram's human-initiated, human-reported scope, and writing the report contract down (`report-contract.md`) makes the human path testable instead of hand-waved. It's the closing half of the two-way Manager ↔ RabbitMQ path the owner confirmed.

- Configure Hermes Desktop to reach the gateway at `http://vm-xa:8080` (LAN); send a request = the single JSON the gateway accepts: `{title, agent_capability, task_brief}`.
- Report delivery: Desktop polls `GET /gateway/reports/<task_id>` (gateway proxies the manager's `/reports`) — record the exact endpoint contract in `deploy/m5/report-contract.md`.

## 3. Ordered full-stack bring-up — `deploy/m5/e2e-deploy.sh`

*Why:* stage-gated bring-up is what makes "the whole stack" a **script, not a ritual** — each stage's healthcheck is the precondition of the next, and any failure halts with the stage named. That naming is the routing rule: the failing stage points at the owning milestone's gate test, so recovery never means re-running the chain.

Idempotent, stage-gated (each stage's healthcheck must pass before the next; any failure halts + prints the stage):

```
S1 chassis     : M0 install.sh (idempotent re-run)
S2 adapter     : M1 compose up + consumer-healthy check
S3 lifecycle   : M2 compose up + deploy gate (pinned images present) + worker registered check
S4 executors   : M3 images load-verified on vm-xb; token issuance dry-run (issue + verify + revoke)
S5 memory/wiki : M4 compose up on vm-hindsight + /healthz check; llm-wiki clone smoke (shallow-clone github.com/arvisdev/llm-wiki from a container — D-wiki-git)
S6 front-house : M5 compose up on vm-xa + gateway /healthz
S7 smoke       : one M2-level task with the stub executor (proves S2–S3 before real agents)
```

```bash
bash deploy/m5/e2e-deploy.sh            # full bring-up; exits non-zero at the failing stage
```

## 4. End-to-end acceptance run (the POC gate)

*Why:* one request, human to human, with the **artifact chain as the evidence** — temporal trace → live labels → commit on origin → report JSON. That chain is what the paper cites as "the POC ran end-to-end"; every link in it is independently checkable after the fact, which is the difference between a demo and a result.

```bash
# E1: one real request, human to human
# Desktop: send {title:"add health endpoint", agent_capability:"dev", task_brief:"..."}
# verify, in order:
ssh vm-xb 'temporal workflow list --type TaskWorkflow | tail -5'          # 1 workflow running
ssh vm-xb 'docker ps --filter label=poc.owner=lifecycle'                 # agent container(s) alive + labeled
ssh vm-xb 'docker ps -a --filter label=poc.owner=lifecycle -q | wc -l'   # after completion: 0 (teardown)
git -C project log --oneline origin/task/<task_id>                        # the commit exists
curl http://vm-xa:8080/gateway/reports/<task_id>                         # report JSON with branch + commit + status
# evidence pack → deploy/m5/evidence/ : temporal trace export, lifecycle log, commit SHA, report payload
```

## 5. Failure-injection runbook (each injection: assert clean reap + human-visible error)

*Why:* five injections are the POC's claim that "ephemeral" survives real failure: a source goes down (F1), the catalog is unreachable (F2), the agent process dies (F3), the TTL is exceeded (F4), and the manager itself restarts mid-task (F5). Each one asserts the same two properties — **zero surviving containers** and an **error the human can see** (F5: the report is still written after the restart) — which is the resilience result, not the hope.

| # | Injection | Command | Expected |
|---|---|---|---|
| F1 | llm-wiki unreachable | block vm-xb network access to the `github.com/arvisdev/llm-wiki` clone path (nftables rule scoped to the repo/443 for the llm-wiki read token) (run task) → remove the block | **policy: fail-fast** — hydrate error (clone failure) ⇒ task fails; container reaped; report = failure + reason |
| F2 | agent catalog unreachable | block vm-xb→GitHub 443 for the read token (nftables) | manager/spawn fails fast; no container leak |
| F3 | agent crash | `docker exec <agent-cid> pkill -9 <agent pid>` | timeout → force-reap; report = failure |
| F4 | TTL exceeded | task with `TTL_S` smaller than work time | reaper path (M2 V3, now with real agents) |
| F5 | manager restart mid-task | start a task; ~10s in: `docker compose restart manager` on vm-xa (§1) | restarted manager reloads its pending set from `/opt/poc/reports` and reconnects its consumer → the report is still written; zero survivors |

After each: `docker ps -a --filter label=poc.owner=lifecycle -q | wc -l` → 0, and the engineer's report shows the error. Record results in `deploy/m5/evidence/failure-injections.txt`.

## 6. Credential hardening (R4 final)

*Why:* this is R4's final form — the negative claim that **no long-lived owner secret is reachable from any container**, verified three ways: trivy across all nine deployed images (the full set the §6a loop iterates), an env sweep of every running container, and the M3 token-lifecycle re-check (401 post-reap). It's the security evidence the paper's threat-model section cites.

```bash
# 6a. negative scan across ALL deployed images:
for i in poc/topic-bridge poc/lifecycle poc/worker poc/executor-stub poc/hermes-ephemeral poc/claude-coder poc/hindsight-api poc/gateway poc/manager; do   # ALL 9 deployed images (llmwiki-api removed per D-wiki-git)
  trivy image --exit-code 1 --scanners vuln,secret $i
done
# 6b. env review: no owner long-lived tokens anywhere:
ssh vm-xb 'for c in $(docker ps -q); do docker inspect --format "{{.Config.Env}}" $c | grep -Ei "owner_?token|github_pat"; done'   # expect empty (filter to .Config.Env, not raw JSON)
# 6c. token lifecycle evidence: M3 §4 B5 re-run (issued at spawn, revoked at reap, 401 after)
```

## 7. Lock the last open items + close the docs

*Why:* the open items (message shapes, concurrency, isolation level, curation owner, wiki hydration shape, VM naming) get **locked or explicitly deferred** here — that's what converts the POC into a Version 2 baseline. VM naming was already settled by the owner 2026-10-25 (POC discussion labels, not commitments). The Version 2 open-items list (curation ownership, Hindsight→LLM-Wiki promotion, long-term queue shape, multi-flow concurrency) is deliberately written *now*, while the answers are fresh.

Decision-log entries (in `deploy/m5/decision-log.md`, mirrored to the open-items section of `../Initial-Foundation-Design-Document.md` §7):

- [x] VM naming (D-8) — resolved 2026-10-25: labels are POC discussion devices, not commitments; per-project naming is a deploy-time choice. POC records its own hostname mapping in `deploy/m0/decision-log.md`.
- [x] TTL ownership — hybrid settled 2026-10-29: `ttl_s` optional in `message-v1.json` (manager sole writer, sized from M3 B1 p50); M2 `POC_TTL_DEFAULT=900s` is the fallback; reaper authoritative either way (M1/M2/M5 docs updated)
- [ ] concurrency model — one-coder-per-task (default) or N-parallel + worktrees (§M3.3)
- [ ] ephemeral isolation level — co-located containers on vm-xb (POC) vs. per-task VMs (defer)
- [ ] llm-wiki curation owner + cadence — tracked in the llm-wiki repo's CHANGELOG.md (from M4; D-wiki-git removed the ingest pipeline, so curation = commits to the repo)
- [ ] **llm-wiki hydration/retrieval shape** (D-wiki-git open item) — interim default: full shallow clone of github.com/arvisdev/llm-wiki at executor boot; a curated index/README-per-topic retrieval aid may be added later (V2)
- [x] **Per-project memory scoping in Hindsight** — resolved 2026-10-30 (D-proj-id): every project has a unique **project-id** identifying its provisioned space in Hindsight; all reads/writes carry it, so no project sees another's memory. V2 implements the project-id dimension (the POC's session/task scoping is the seed).
- [ ] **Shared-VM capacity + firewall governance** — the shared Hindsight VM serves every project pair: aggregate the per-project ≤5-container caps for sizing, and govern each project's UFW allow-list entry (the POC's `UFW_POC_CIDR` rule is the single-project seed)
- [ ] **llm-wiki content taxonomy** — D-wiki-truth sets the split: per-project reference area keyed by project-id + shared enterprise area (company design specifications and the like); what remains open is the curation owner + cadence (tracked in the repo's CHANGELOG, from M4) and the per-project onboarding flow
- [ ] **Multi-host git support** — the design is host-agnostic (repos are URLs; `repo_target`/catalog/llm-wiki clone the same way), but credential issuance is currently GitHub-only (`deploy/m3/issue-token.sh`: GitHub App JWT → installation token). A second host (ADO, Bitbucket) needs its own short-lived-token issuer behind the same FR-S1 contract. Trigger: any adopting project whose repos live outside GitHub; motivation to consolidate hosts instead is a client/org choice, not an architecture requirement.
- [x] **Wiki-vs-repo boundary** — resolved 2026-10-30 (D-wiki-truth/D-repo-truth): development- and business-related items relevant to the deliverables → the project repo (GitHub/ADO, per project); **reference material → the llm-wiki repo**. Agents apply this rule when deciding where a document lives. (2026-11-04, D-wiki-git: the wiki is the git repo github.com/arvisdev/llm-wiki.)

Then update `../ARCHITECTURE.md` + `../Initial-Foundation-Design-Document.md`: move resolved items out of "open", add the **Version 2 open items list** (llm-wiki curation ownership, Hindsight→llm-wiki promotion, wiki hydration/retrieval shape, long-term queue shape, multi-flow concurrency).

## 8. Rollback

*Why:* M5 adds **no new infrastructure**, so its rollback is front-house-only — `compose down` on vm-xa, and everything behind it (M0–M4) stays up. When E2E fails, the failing stage names the owning milestone and you re-run *that* milestone's gate test, never the whole chain.

M5 adds no new infrastructure. If E2E fails, the failing stage in §3 names the owning milestone — re-run **that milestone's** gate test (M1 §4, M2 §3, M3 §4, M4 §4), not the whole chain. Rollback of the front-house only:

```bash
ssh vm-xa 'cd /opt/poc/deploy/m5 && docker compose down'
# everything behind it (M0–M4) stays up and is untouched
```

## 9. POC complete — done-when (all five)

*Why:* five concrete conditions make "the POC is complete" a **verifiable claim** instead of a feeling — and when they hold, the verdict in the feasibility analysis is confirmed *by evidence, not by analysis*. That sentence is the paper's thesis, and these five lines are its proof.

1. §4 E2E run passed, **zero** surviving POC containers.
2. §5 all four injections clean + reported.
3. §6 scans pass; no long-lived owner secret reachable from any container.
4. §7 all open items locked in the decision log; docs updated.
5. Engineer sees the outcome (report JSON with artifact pointer) — the diagram's human-initiated, human-reported scope is closed.

When 1–5 hold, the verdict in `../Technical-Feasibility-Analysis.md` is confirmed **by evidence, not by analysis**.
