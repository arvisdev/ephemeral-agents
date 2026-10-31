# schemas/ — the live inter-zone contracts

Versioned by **filename**, not by directory: `message-v1.json` is the V1 task
message the manager publishes and the M1 topic-bridge consumes; a future V2
contract would be added here as `message-v2.json`, alongside — not replacing —
the V1 file until no V1 consumer remains.

Rules:

1. **Authoritative copies live here.** Any copy elsewhere (e.g.
   `services/topic-bridge/schemas/`) must be byte-identical; the M1 install
   script's asset check compares them and fails on divergence (drift guard).
2. **This directory serves the current iteration.** At a V2 kickoff, `deploy/`
   and `services/` freeze as the V1 evidence trail (see the README's
   "Versioning conventions") — V1 schema files are retired only when no V1
   consumer remains.
