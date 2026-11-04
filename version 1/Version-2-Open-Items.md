# Version 2 Open Items List

*The consolidated carry-over list required at M5 exit (BRD BO-7, WP-5.6,
TC-M5-07). Each item states what the POC proved, what it did not decide, and
what V2 must answer. Created 2026-11-04, pre-kickoff; M5 exit confirms or
extends it with live-deployment findings.*

## 1. Ephemeral isolation level — containers vs. per-task VMs

- **What the POC proved:** ephemeral Docker containers on the shared vm-xb
  can be spawned, TTL-bounded, and reaped leak-free (M2/M5 gates), with
  per-run scoped credentials.
- **What it did not decide:** whether container-level isolation is a
  *sufficient* security boundary for production work. A compromised agent on
  a shared host is a different risk class than one on its own disposable VM.
- **V2 must answer:** the isolation boundary per agent type (general agent
  vs. coder), driven by the credential and data classes each touches; the
  boot-latency and capacity cost of per-task VMs if chosen (feeds the TTL
  model); and the decision recorded against BR-5 (no shared architecture
  with production) — V2 may diverge here deliberately.

## 2. Knowledge promotion flows (wiki ↔ Hindsight ↔ GitHub)

- **What the POC proved:** agents write explicitly-scoped learnings to
  Hindsight (`cross-task`) and read the llm-wiki; promotion of a standard
  into `behavioral-spec.md` is a HITL act (Store-Usage-Rules).
- **What it did not decide:** the two unlabeled arrows in the source diagram
  (LLM-Wiki→Hindsight, LLM-Wiki→GitHub). Today all knowledge movement is
  manual; whether any of it becomes automatic — and with what review gate —
  is open.
- **V2 must answer:** which promotions (if any) are automated; the review
  gate for each (unreviewed auto-promotion would break the truth chain:
  git = exact, Hindsight = never the authority); and whether agents may
  propose (PR-style) rather than write directly.

## 3. Multi-project scale on shared stores (D-mp-shared consequences)

- **What the POC proved:** one project's flow end-to-end; project-id scoping
  of Hindsight enforced by the API's query contract; per-project VM pairs.
- **What it did not decide:** the shared Hindsight and llm-wiki at many
  projects.
- **V2 must answer:** memory-scoping enforcement at scale (query-filter
  convention today — what enforces it against a buggy or hostile client at
  50 projects: row-level policy, per-project DB, or API gateway?); capacity
  planning for the shared memory VMs; and firewall/allow-list governance as
  per-project vm-xa/vm-xb pairs multiply.

## 4. The RabbitMQ ingestion boundary's other side (D-rmq)

- **What the POC proved:** the topic exchange + bridge is a durable,
  idempotent ingestion path (M1 gates) — kept deliberately as the V2
  boundary.
- **What it did not decide:** what may publish to `agent.tasks` besides the
  manager. V1 has exactly one publisher and no authentication story beyond
  the broker credential.
- **V2 must answer:** which other producers inject work (CI, schedulers,
  other tools); the message/authenticity contract for them (per-producer
  credentials? signed envelopes? schema v2 with a producer field); and the
  admission policy (per-project routing keys vs. shared exchange).

## Resolved during V1 (recorded here so V2 does not re-open them)

- VM naming (D-8, 2026-10-25) · language for Temporal code (D-lang) ·
  RabbitMQ boundary kept (D-rmq, 2026-10-29) · hybrid TTL model (D-ttl) ·
  message shape v1, task_id as the single idempotency key (D-msg-v1) ·
  catalog layout + spec home (D-catalog, D-spec-source) · llm-wiki = git
  repo, VM dropped (D-wiki-git, 2026-11-04) · hydration shape (D-hydrate) ·
  embedding provider (D-embed) · curation owner/cadence (D-curation) ·
  sign-off authority (D-signoff) · manager reporting resilience (F5 gate).

## Not carried (explicitly out of scope for V2 unless reopened)

- Load-handling/concurrency *semantics beyond the ≤5 cap* (BRD open item —
  capacity engineering, not architecture; revisit only if demand appears).
- Wiki curation tooling beyond git (the HITL + git workflow suffices until
  volume says otherwise).
