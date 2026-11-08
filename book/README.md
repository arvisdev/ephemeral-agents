# The Ephemeral Agents Book

*An organization of AI agents — the thesis, the design, and the proof.*

This book presents the ephemeral-agents system in three parts, written for
two audiences at once: decision-makers who need the idea in plain language,
and engineers who need the decisions, alternatives, and evidence. Every
chapter opens with an **In plain terms** box — a reader who needs only the
essence can read the boxes and the section headings; the body beneath each
is the technical treatment.

**The running example.** One request — *"market analysis on the feasibility
of applying stock market results against profit margin historical data"* —
is introduced as a human problem in Chapter 1, carried through every
component in Part II, and fully traced in Part III.

## Part I — The Thesis (accessible; no jargon)

| Chapter | Title | What it covers |
|---|---|---|
| 1 | [The Problem](ch1-the-problem.md) | The seven walls of ad-hoc agent adoption — continuity, trust, accountability, specialization, reliability, cleanup, understanding |
| 2 | [The Thesis](ch2-the-thesis.md) | The claim, stated precisely; six principles (P1–P6); testable predictions; the honest benefit frame |
| 3 | [The Organization](ch3-the-organization.md) | Three departments (business, engineering, workforce), the four stages of work, the three human positions of authority, the proposer/verifier split |

## Part II — The Practical Design (narrative + engineering depth)

| Chapter | Title | What it covers |
|---|---|---|
| 4 | [The Work Order](ch4-the-work-order.md) | The message contract field by field; the self-sufficiency rule; rejected alternatives |
| 5 | [The Durable Core](ch5-the-durable-core.md) | The workflow engine, the queue, the lifecycle; two clocks; the zero-survivors guarantee |
| 6 | [The Roster](ch6-the-roster.md) | Role files and the machine index; how roles are authored; role vs body separation |
| 7 | [The Executors](ch7-the-executors.md) | Two harnesses (general agent, code executor); the hydration contract; the boot self-report |
| 8 | [Credentials](ch8-credentials.md) | Per-task identity; issuance and verified revocation; what each component may hold |
| 9 | [Memory](ch9-memory.md) | Organization-owned memory; read and write contracts; degradation over dependency |
| 10 | [The Front Door](ch10-the-front-door.md) | The thinking/execution boundary; gateway and registrar; the Chief of Staff as proposer, code as disposer |

## Part III — The Proof: lessons from the build

| Chapter | Title | What it covers |
|---|---|---|
| 11 | [The Build](ch11-the-build.md) | Six stages over four days, the gate rules, what was measured |
| 12 | [The Failure Ledger](ch12-the-failure-ledger.md) | ~35 defects in five classes; ten stories that earned design rules; process lessons |
| 13 | [What Remains](ch13-what-remains.md) | What was demonstrated vs deferred; the next version in priority order; the verdict |
| 14 | [Building from Scratch](ch14-building-from-scratch.md) | The operator's chapter: intake list, estate inventory, VM provisioning, the idempotent deployment sequence, secrets staging, timeline |

## Conventions

- **Labels with permanent meaning:** Walls 1–7 (Chapter 1) and Principles
  P1–P6 (Chapter 2) are cited by number throughout.
- **Sensitive data:** the book contains no credentials, tokens, keys, or
  personal data. The owner's public account name
  (`github.com/arvisdev/...`) and internal `192.168.0.x` LAN addresses are
  approved for inclusion by the owner and appear as-is.
- **Evidence:** every measured claim in Part III traces to the project's
  filed evidence pack; the re-runnable material is committed in this
  repository.

*Companion documents (repository root): `Initial-POC-Findings-Report.md`
(the evidence record) and `POC-Retrospective-and-Insights.md` (the
practitioner's lessons digest).*
