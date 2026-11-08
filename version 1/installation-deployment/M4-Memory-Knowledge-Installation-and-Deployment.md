# M4 Installation & Deployment — Agent Memory & Knowledge Access (Hindsight + llm-wiki)

**Companion:** `../delivery plans/M4-Memory-and-Knowledge-Delivery-Plan.md` · this file: **how to install and deploy it**.

| | |
|---|---|
| **Installs on** | vm-hindsight (Hindsight: Postgres+pgvector + API) — *(D-wiki-git, 2026-11-04: no wiki VM; llm-wiki = the git repo `github.com/<organization>/llm-wiki`, consumed by shallow clone)* |
| **Prereqs** | M0 (vm-hindsight exists, Docker up), M2/M3 (an executor that hydrates and writes memory) |
| **What gets deployed** | `poc/hindsight-api:1.0.0` + `hindsight-pg` (vm-hindsight), updated M2/M3 env wiring (llm-wiki repo URL for the clone) |
| **Language (mandatory)** | No new Temporal code in this milestone — all workflow/activity code stays in the M2 Go services (owner instruction: Go for all Temporal code, speed + size). The Hindsight API service follows the POC's Go convention. |

## 1. Hindsight on vm-hindsight

> **Sizing note:** the M0 baseline for this VM (4 GB / 2 cores / 20 GB) is a minimum. Recommended: 4–8 GB RAM (pgvector wants RAM for the index) and 40 GB disk (20 GB fills over a multi-day POC with embeddings). See the sizing table in M0 section 1.

*Why:* Hindsight is the diagram's "R/W memory" made concrete — a small Go API over Postgres+pgvector (one image, so the `vector` extension can't be missing). Its Postgres instance is **separate from Temporal's by rule** (M0 section 4), so a memory failure can never take down the workflow engine, and vice versa. That isolation is re-verified at every M4 gate (TC-M4-07).

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
    image: poc/hindsight-api:1.0.0       # small Go service (built in section 1b)
    environment:
      DB_DSN: postgres://hindsight:${HINDSIGHT_DB_PASS}@hindsight-pg:5432/hindsight
      LISTEN: 0.0.0.0:8081
      EMBEDDINGS: ${EMBEDDING_PROVIDER:-none}        # D-embed: "http" (OpenAI-compatible) — "none" = zero-vector fallback
      EMBEDDING_API_KEY: ${EMBEDDING_API_KEY:-}      # provider key (D-embed) — absent ⇒ zero-vector fallback, reads degenerate
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
  embedding vector(1536),     -- dimension per provider (1536 = OpenAI text-embedding-3-small);
                              -- changing provider/dimension forces a table rebuild — record it
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
ssh vm-hindsight 'mkdir -p /opt/poc/deploy/m4/hindsight'
rsync -a deploy/m4/hindsight/ vm-hindsight:/opt/poc/deploy/m4/hindsight/
ssh vm-hindsight 'cd /opt/poc/deploy/m4/hindsight && docker compose up -d'
```

## 2. llm-wiki — the git repo (no deployment)

*Why:* per **D-wiki-git (owner, 2026-11-04)** there is **no wiki VM and no wiki service** — the former LLM-Wiki (vm-llm-wiki, `llmwiki-api`, pgvector serving layer, ingest pipeline) is removed entirely. Reference standards live in a plain, private git repository that consumers shallow-clone; the content repo IS the store (D-wiki-truth, now trivially true).

**2a. The repo:** github.com/<organization>/llm-wiki — already created and scaffolded:

```
llm-wiki/  (github.com/<organization>/llm-wiki, private)
  design-standards/      *.md   (architecture standards, naming, patterns)
  dev-best-practices/    *.md
  business-best-practices/*.md
  tools-and-skills/      *.md   (tool docs the agents may need)
  CHANGELOG.md           (curation cadence + ownership note — tracked here, not in a decision log service)
  README.md
```

**2b. Consumption:** hydrate-time `git clone --depth 1` of the repo. Read access uses a **distinct static `LLM_WIKI_READ_TOKEN`** — a fine-grained personal access token granting `contents:read` on `<organization>/llm-wiki` only (not a GitHub App installation token, and not the catalog's `GITHUB_READ_TOKEN`). Nothing to install on any VM for this.

- **Open item (recorded, not solved):** the hydration/retrieval shape is undecided — the interim default is a **full shallow clone at executor boot**; agents read the markdown directly. A curated index / README-per-topic retrieval aid may be added later (V2).
- Curation for POC = markdown committed to the repo; every clone made after the push sees it (no reindex step — the ingest pipeline is gone).

## 3. Wire hydration into the lifecycle + executors (the connective work)

*Why:* this is the connective tissue, and it's deliberately **config, not new Temporal code** — `HINDSIGHT_URL` and the llm-wiki repo URL ride into every spawned container as env, and the M2 Go worker hydrates (read + clone) and writes-on-end without a code change. That's the Go mandate paying off: adding memory + knowledge to the flow touched zero Temporal code.

1. **Names:** ensure `vm-hindsight` resolves on vm-xb/vm-xa (section 5 of M0 — `/etc/hosts` or PVE DNS; same mechanism). No wiki hostname exists (D-wiki-git).
2. **M2 config** (`deploy/m2/compose.yaml` env + lifecycle config): add `HINDSIGHT_URL=http://vm-hindsight:8081` and the llm-wiki clone settings (`LLM_WIKI_REPO_URL=github.com/<organization>/llm-wiki` + `LLM_WIKI_READ_TOKEN`) → these ride into every spawned container as env (the executor entrypoints from M3 section 2a/2b already read `HINDSIGHT_URL`/`LLM_WIKI_*`).
3. **Hydrate step** (lifecycle worker): before task work, fan out the llm-wiki **shallow clone** (`git clone --depth 1 $LLM_WIKI_REPO_URL`) + `memory/read` → pass results into the agent's task payload; record per-source latency.
4. **Write-on-end:** at task completion, `memory/write` the agent's report (scope per the section 1 contract: artifacts/learnings = `cross-task`, run scratch = `session`).
5. Redeploy lifecycle + worker with the new env; verify pre-pull/gate still passes.

## 4. Verification (the M4 gate tests)

*Why:* the **cross-task read-back (G2) is the single property that distinguishes "long-term memory" from a per-task scratchpad** — task t1's learning must be visible to task t2. G5's latency check protects the M2 TTL headroom (hydration must not slow spawn+work past the TTL). Evidence files are what `TP-M4` signs.

```bash
# G1 network R/W: from a spawned agent container:
curl -s "$HINDSIGHT_URL/memory/read?task_id=t1&scope=cross-task" ; curl -s -XPOST "$HINDSIGHT_URL/memory/write" -d '{...}'
# G2 cross-task read-back: task t1 writes {learning:"use X for Y"}; task t2's hydrate fetches it
# G3 llm-wiki clone + read: from a spawned agent container (D-wiki-git — replaces the former wiki-query check):
git clone --depth 1 "$LLM_WIKI_REPO_URL" /tmp/llm-wiki && ls /tmp/llm-wiki/design-standards/
# → 3 canned lookups (a standard, a skill, a business practice) each find the right doc file/section in the clone (record in evidence)
# G4 catalog regression: M3's self-report still correct with the two new sources live
# G5 latency: hydration per-source + total recorded (clone + memory read); total < M2 TTL headroom (measure 3 runs)
```

Evidence → `deploy/m4/evidence/` (read-back rows, query results with hits, latency table).

## 5. Rollback / re-do

*Why:* named volumes keep rollback **non-destructive by default** (`down` keeps data; only a true re-do removes volumes). And because executors tolerate empty `HINDSIGHT_URL`/`LLM_WIKI_*` (hydrate skips missing sources), pulling M4 back **degrades** hydration rather than breaking the flow — the resilience property, stated.

```bash
# services: down per-VM compose project; data lives in named volumes:
docker compose down                     # keeps hindsight-pg-data
docker volume rm hindsight-pg-data      # only on a true re-do
# wiring: reverting the M2 env block is a config revert; executors tolerate empty HINDSIGHT_URL/LLM_WIKI_* (hydrate skips missing sources)
```

## 6. Troubleshooting

*Why:* the two rows worth memorizing: "extension vector not installed" (wrong image — the pgvector pin from section 1a) and "cross-task read-back empty" (the **scope-rule mismatch**: written with `scope=session` where `cross-task` was intended). Both are config/contract errors that G1–G2 catch in seconds if you run them first.

| Symptom | Cause / fix |
|---|---|
| llm-wiki clone fails or times out in hydrate (G3) | repo URL/read token wrong, or egress to github.com blocked from the container — check `LLM_WIKI_REPO_URL` + `LLM_WIKI_READ_TOKEN` scope (`contents:read` on arvisdev/llm-wiki is enough); a clean clone failure must surface as a hydrate error (see M5 F1), never silently skipped |
| Wrong doc found by an agent (G3 content miss) | with a plain clone there is no retrieval layer — content navigation is the repo's own README/directory layout; if lookups miss, fix the repo's index/README (the V2 curated-index open item), not the executor |
| `extension "vector" is not installed` | used `postgres:16` instead of `pgvector/pgvector:pg16` (section 1a image pin) |
| API 500 on write | embedding provider key missing/rate-limited — check `EMBEDDINGS` env + provider quota (rate-limit note from the owner's standing conventions) |
| Hydration slow | clone latency in the hot path — POC-scale: the repo is small; check network + token issuance latency; record actuals (section 4 G5) |
| `vm-hindsight:8081` unresolvable from vm-xb | section 3.1 name mechanism not applied to every VM that spawns/consumes |
| Stale standards after content push | not possible at the service layer (none exists) — stale clones are fixed by re-cloning at the next boot; if a run must see a fresh push, re-clone before work (curation cadence note lives in the repo's CHANGELOG) |
| Cross-task read-back empty | scope rule mismatch: write used `scope=session` where `cross-task` was intended — check the executor write (section 3.4) |
