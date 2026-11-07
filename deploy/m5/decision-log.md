# M5 decision log — lock the open items

- [ ] VM numbering (carried from M0) — final scheme recorded
- [ ] topic/queue message shapes — schemas/message-v1.json is the contract (version it)
- [ ] concurrency model — one-coder-per-task (default) or N-parallel + worktrees (M3 §3)
- [ ] ephemeral isolation level — co-located containers on vm-xb (POC) vs. per-task VMs (defer)
- [x] llm-wiki repo curation owner + cadence (from M4) — D-curation (2026-11-04): owner = the Human-In-The-Loop (HITL); append-only, need-driven; record in the llm-wiki repo's CHANGELOG.md

After locking: update version 1/ARCHITECTURE.md + Initial-Foundation-Design-Document.md
(move resolved items out of "open") and add the Version 2 open items list
(llm-wiki repo curation ownership, long-term queue shape,
multi-flow concurrency).
