# M3 Delivery Plan — Agent Executor Proofs (Hermes Ephemeral + Claude Code Headless)

| | |
|---|---|
| **Milestone** | M3 (from `../Technical-Feasibility-POC-Milestones.md`) |
| **Goal** | Prove the two To-verify components inside the M2 lifecycle: **M3a** a Hermes agent container that boots with persona/skills loaded from the Available Agent List repo; **M3b** a headless Claude Code container that makes a real code change and commits it with a scoped token. |
| **Depends on** | M2 (lifecycle manager) |
| **Retires risk** | R5 (Hermes persona-load at boot), R3 + R4 (concurrency model, credential scoping) |

This is the **point of last cheap exit** in the whole POC: if a spike fails here, it's still a two-week fix, not a two-month one.

## M3a — Hermes Agent (Ephemeral)

**WP-3a.1 — Agent catalog layout (the "Available Agent List" repo)**
- Define the repo layout matching Note 1: an **agent index** (manifest of available agents, e.g. `index.json` or `index.md`) + per-agent folders containing the full persona markdown, skill and tools files, and the behavioral specification reference.
- This layout is the contract the Manager Agent searches (Note 1: "look in the 'Available Agent List' repo and search the agent index"). Deliverable: the repo itself with one fully-worked example agent.

**WP-3a.2 — Ephemeral Hermes image**
- Build the image: runtime + Hermes + a boot entrypoint that, from per-run config (M2's `agent_ref` + repo folder), clones/pulls the repo and loads the agent's persona + skills **at startup** — stateless, no prior state assumed (Note 2: "pull the full agent profile markdowns… load those profiles… then destroy").
- Boot checks: the agent self-reports its role, loaded skills, and received behavioral spec before it does task work.

**WP-3a.3 — Boot-time and failure measurement**
- Record cold boot → "ready" time; target: bounded and reproducible (measure, don't guess; decide the limit after 3 runs).
- Failure paths: repo unreachable, malformed persona, missing index entry → clean error surfaced to the workflow + container reaped (no hang past M2's TTL).

## M3b — Claude Code Agent (N+1)

**WP-3b.1 — Headless coder image**
- Image: base + Node + git + Claude Code CLI + toolchain for the POC's target project language. Pinned tag, pre-pulled per M2's no-pull rule.
- Entry: clone/checkout the Project Repo work dir → receive the task (behavioral spec + work item) → make the change → commit + push a per-task branch.

**WP-3b.2 — Scoped credentials (R4)**
- Short-lived, least-privilege GitHub credentials only: per-run token (fine-grained PAT or GitHub App install token) scoped to the **single Project Repo** and **one task branch**. No long-lived owner token anywhere in the image or env; injected at spawn, gone at reap.
- Deliverable: token-issuance note (who issues, TTL, revocation on reap) + a scan proving no static secret in the image layers or container env snapshot.

**WP-3b.3 — Concurrency model spike (R3)**
- Decide and record: **one coder per task** (default, simplest: one container, one task branch) vs. **N parallel coders** (needs git worktrees/branches + merge strategy).
- Run the chosen model: e.g. 3 sequential single-coder tasks and (if N-parallel is chosen) 2 concurrent coders on the same repo via worktrees → all commits land, no lost work.

## Deliverables

1. Two pinned executor images (M3a, M3b) replacing M2's placeholders; deploy config updated.
2. The Available Agent List repo (WP-3a.1) with one worked example agent.
3. Scoped-token mechanism + the negative secret scan (WP-3b.2).
4. Spike report: boot times, one real M3a run (agent acting in its persona), one real M3b run (an actual commit in the Project Repo on a task branch), and the concurrency decision with evidence.

## Acceptance criteria (done-when)

1. **M3a:** container boots, self-reports the correct persona/skills/spec from the repo; boot time recorded and reproducible across 3 runs.
2. **M3a failure paths:** all three fault injections end in clean error + reap.
3. **M3b:** a real code change is committed + pushed to a task branch using a scoped token; token is unusable after reap (revoked/expired).
4. **R3 recorded:** the concurrency model decision is in the decision log with the evidence run attached.

## Exit gate

M4 may start when criteria 1–4 pass. **If M3a's boot-time persona-load fails and the sidecar fallback (a hydration sidecar that prepares the agent config before the agent process starts) also fails, stop the POC sequence** and re-baseline the executor design — that is the R5 abort condition.

## Notes

- The (N+1) second-container fan-out from the "Executes Activity" diamond is implemented here only to the extent the concurrency decision requires it; keep the default single-coder path primary.
- Everything M3 produces is **spike-quality with production-shaped plumbing**: real images, real repo layout, real credentials model — so M5's end-to-end run reuses it unchanged.
