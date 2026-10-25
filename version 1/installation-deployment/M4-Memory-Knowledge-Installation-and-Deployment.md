# M4 Installation & Deployment — Memory + Knowledge (Hindsight + LLM-Wiki)

**Companion:** `../delivery plans/M4-Memory-and-Knowledge-Delivery-Plan.md` · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | vm-3 (Hindsight: Postgres+pgvector + API), vm-2 (LLM-Wiki: content + retrieval API + ingest) |
| **Prereqs** | M0 (vm-2/vm-3 exist, Docker up), M2/M3 (an executor that hydrates and writes memory) |
| **What gets deployed** | `poc/hindsight-api:1.0.0` + `hindsight-pg` (vm-3), `poc/llmwiki-api:1.0.0` + content repo + ingest job (vm-2), updated M2/M3 env wiring |
| **Language (mandatory)** | No new Temporal code in this milestone — all workflow/activity code stays in the M2 Go services (owner instruction: Go for all Temporal code, speed + size). The Hindsight/LLM-Wiki API services follow the POC's Go convention. |

## 1. Hindsight on vm-3

*Why:* Hindsight is the diagram's "R/W memory" made concrete — a small Go API over Postgres+pgvector (one image, so the `vector` extension can't be missing). Its Postgres instance is **separate from Temporal's by rule** (M0 §4), so a memory failure can never take down the workflow engine, and vice versa. That isolation is re-verified at every M4 gate (TC-M4-07).

**1a. Stack — `deploy/m4/hindsight/compose.yaml`:**

```yaml
services:
  hindsight-pg:
    image: pgvector/pgvector:pg16        # Postgres + pgvector extension in one image (pin)
    environment:
      POSTGRES_DB: hindsight
      POSTGRES_USER: hindsight
      POSTGRES_PASSWORD: ${HINDSIGHT_DB_PASS}
    volumes:
      - hindsight-pg-data:/var/lib/postgresql/data
      - ./initdb.sql:/docker-entrypoint-initdb.d/01-init.sql:ro   # CREATE EXTENSION vector; tables below
    healthcheck: {test: ["CMD-SHELL", "pg_isready -U hindsight"], interval: 10s}
  hindsight-api:
    image: poc/hindsight-api:1.0.0       # small Go service (built in §1b)
    environment:
      DB_DSN: postgres://hindsight:${HINDSIGHT_DB_PASS}@hindsight-pg:5432/hindsight
      LISTEN: 0.0.0.0:8081
      EMBEDDINGS: ${EMBEDDING_PROVIDER}  # provider API at POC scale (record the choice + rate-limit note)
    depends_on: {hindsight-pg: {condition: service_healthy}}
    restart: unless-stopped
volumes:
  hindsight-pg-data: {}
```

`initdb.sql` (schema — the minimal POC contract):

```sql
CREATE EXTENSION IF NOT EXISTS vector;
CREATE TABLE memory_entries (
  task_id   text,
  agent_ref text,
  scope     text,              -- 'session' | 'cross-task' (the scoping rule from the plan)
  body      jsonb,
  embedding vector(1536),     -- dimension per provider; record the actual value
  created   timestamptz default now()
);
```

**1b. API service** (Go, `services/hindsight-api/`) — the concrete meaning of the diagram's "R/W Memory":

- `GET /memory/read?task_id=&scope=&q=` → top-k relevant entries (exact-scope rows + vector similarity over `cross-task`)
- `POST /memory/write` → `{task_id, agent_ref, scope, body}` → embed `body`, insert
- `GET /healthz`
- Client helper: `services/hindsight-api/client/` (Go package + a `curl` example) — this is what the executor entrypoints use.

```bash
docker build -f deploy/m4/hindsight/Dockerfile.api -t poc/hindsight-api:1.0.0 services/hindsight-api
ssh vm-3 'mkdir -p /opt/poc/deploy/m4/hindsight' && cp -r deploy/m4/hindsight/* / via rsync
cd /opt/poc/deploy/m4/hindsight && docker compose up -d
```

## 2. LLM-Wiki on vm-2

*Why:* LLM-Wiki is the owner's **curation artifact** (R7) — the whole process is "owner commits markdown, `docker compose run llmwiki-ingest` reindexes", and the curation cadence + owner is recorded in the decision log instead of drifting. Holding design standards and dev/business best practices here (not in the Git repo) follows the owner-confirmed store-content model.

**2a. Content repo (owner-curated, the curation artifact for R7):** `llmwiki-content/`:

```
llmwiki-content/
  design-standards/      *.md   (architecture standards, naming, patterns)
  dev-best-practices/    *.md
  business-best-practices/*.md
  tools-and-skills/      *.md   (tool docs the agents may need)
  CHANGELOG.md           (curation cadence: "reindex on push" — record the owner + cadence in decision log)
```

**2b. Stack — `deploy/m4/llmwiki/compose.yaml` (vm-2):**

```yaml
services:
  llmwiki-pg:
    image: pgvector/pgvector:pg16        # separate instance (same rule as M0: never share DBs across VMs)
    environment:
      POSTGRES_DB: llmwiki
      POSTGRES_USER: llmwiki
      POSTGRES_PASSWORD: ${LLMWIKI_DB_PASS}
    volumes: [llmwiki-pg-data:/var/lib/postgresql/data]
  llmwiki-api:
    image: poc/llmwiki-api:1.0.0         # Go service: retrieval + ingest endpoints
    environment:
      DB_DSN: postgres://llmwiki:${LLMWIKI_DB_PASS}@llmwiki-pg:5432/llmwiki
      EMBEDDINGS: ${EMBEDDING_PROVIDER}
      LISTEN: 0.0.0.0:8080
    ports: ["8080"]
    depends_on: {llmwiki-pg: {condition: service_healthy}}
  llmwiki-ingest:
    image: poc/llmwiki-api:1.0.0
    command: ["ingest"]                  # one-shot: clone llmwiki-content (depth 1), chunk, embed, upsert
    environment:
      DB_DSN: postgres://llmwiki:${LLMWIKI_DB_PASS}@llmwiki-pg:5432/llmwiki
      CONTENT_REPO: git@github.com:<owner>/llmwiki-content.git
      GITHUB_TOKEN: ${LLMWIKI_READ_TOKEN}
      EMBEDDINGS: ${EMBEDDING_PROVIDER}
    volumes: [llmwiki-pg-data:/var/lib/postgresql/data]   # upsert path
volumes:
  llmwiki-pg-data: {}
```

Endpoints:

- `GET /wiki/query?q=<design standards|skill|topic>&k=5` → passages with source file + section
- `POST /wiki/ingest` (or run `llmwiki-ingest` manually) → reindex from the content repo
- Curation for POC = **markdown committed to the repo → `docker compose run llmwiki-ingest`** (documented in `CHANGELOG.md`; the "who curates" answer lands in the decision log — R7).

## 3. Wire hydration into the lifecycle + executors (the connective work)

*Why:* this is the connective tissue, and it's deliberately **config, not new Temporal code** — `HINTSIGHT_URL`/`LLMWIKI_URL` ride into every spawned container as env, and the M2 Go worker hydrates (read) and writes-on-end without a code change. That's the Go mandate paying off: adding memory + knowledge to the flow touched zero Temporal code.

1. **Names:** ensure `vm-2`/`vm-3` resolve on vm-xb/vm-xa (§5 of M0 — `/etc/hosts` or PVE DNS; same mechanism).
2. **M2 config** (`deploy/m2/compose.yaml` env + lifecycle config): add `HINTSIGHT_URL=http://vm-3:8081`, `LLMWIKI_URL=http://vm-2:8080` → these ride into every spawned container as env (the executor entrypoints from M3 §2a/2b already read `HINTSIGHT_URL`/`LLMWIKI_URL`).
3. **Hydrate step** (lifecycle worker): before task work, fan out `wiki/query` (topic = task + agent_ref) + `memory/read` → pass results into the agent's task payload; record per-source latency.
4. **Write-on-end:** at task completion, `memory/write` the agent's report (scope per the §1 contract: artifacts/learnings = `cross-task`, run scratch = `session`).
5. Redeploy lifecycle + worker with the new env; verify pre-pull/gate still passes.

## 4. Verification (the M4 gate tests)

*Why:* the **cross-task read-back (G2) is the single property that distinguishes "long-term memory" from a per-task scratchpad** — task t1's learning must be visible to task t2. G5's latency check protects the M2 TTL headroom (hydration must not slow spawn+work past the TTL). Evidence files are what `TP-M4` signs.

```bash
# G1 network R/W: from a spawned agent container:
curl -s "$HINTSIGHT_URL/memory/read?task_id=t1&scope=cross-task" ; curl -s -XPOST "$HINTSIGHT_URL/memory/write" -d '{...}'
# G2 cross-task read-back: task t1 writes {learning:"use X for Y"}; task t2's hydrate fetches it
# G3 retrieval quality: 3 canned queries (a standard, a skill, a business practice) → top-1 hit is the right doc section (record in evidence)
# G4 catalog regression: M3's self-report still correct with the two new sources live
# G5 latency: hydration per-source + total recorded; total < M2 TTL headroom (measure 3 runs)
```

Evidence → `deploy/m4/evidence/` (read-back rows, query results with hits, latency table).

## 5. Rollback / re-do

*Why:* named volumes keep rollback **non-destructive by default** (`down` keeps data; only a true re-do removes volumes). And because executors tolerate empty `HINTSIGHT_URL`/`LLMWIKI_URL` (hydrate skips missing sources), pulling M4 back **degrades** hydration rather than breaking the flow — the resilience property, stated.

```bash
# services: down per-VM compose project; data lives in named volumes:
docker compose down                     # keeps hindsight-pg-data / llmwiki-pg-data
docker volume rm hindsight-pg-data llmwiki-pg-data   # only on a true re-do
# wiring: reverting the M2 env block is a config revert; executors tolerate empty HINTSIGHT_URL/LLMWIKI_URL (hydrate skips missing sources)
```

## 6. Troubleshooting

*Why:* the two rows worth memorizing: "extension vector not installed" (wrong image — the pgvector pin from §1a) and "cross-task read-back empty" (the **scope-rule mismatch**: written with `scope=session` where `cross-task` was intended). Both are config/contract errors that G1–G2 catch in seconds if you run them first.

| Symptom | Cause / fix |
|---|---|
| `extension "vector" is not installed` | used `postgres:16` instead of `pgvector/pgvector:pg16` (§1a image pin) |
| API 500 on write | embedding provider key missing/rate-limited — check `EMBEDDINGS` env + provider quota (rate-limit note from the owner's standing conventions) |
| Hydration slow | embedding calls in the hot path — POC-scale: precompute nothing, but check provider latency and `k`; record actuals (§4 G5) |
| `vm-3:8081` unresolvable from vm-xb | §3.1 name mechanism not applied to every VM that spawns/consumes |
| Stale standards after content push | ingest not re-run — `docker compose run llmwiki-ingest` (that's the documented cadence, not a bug) |
| Cross-task read-back empty | scope rule mismatch: write used `scope=session` where `cross-task` was intended — check the executor write (§3.4) |
