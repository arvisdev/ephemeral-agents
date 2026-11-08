# Chapter 7 — The Executors

> **In plain terms.** The roster defines *who* does the work; the executor is
> the *body* they occupy while doing it. This chapter explains why there are
> exactly two executor "harnesses" — not one, not twenty-two — what each is
> for, and how a generic body becomes a specific specialist within about a
> second of being born.

## 7.1 The separation: role ≠ body

A design temptation, early on, was to build one container image per role:
an "image for the backend engineer," an "image for the risk analyst," and
so on. The build rejected it, and the reasoning is worth keeping because it
shapes everything in this chapter.

A role (Chapter 6) is *knowledge and behavior* — prose, skills, an operating
spec. A body is *capability and plumbing* — an LLM, a tool runtime, a file
system, network access, the code that boots and shuts down. Roles change
weekly (new text); bodies change rarely (new runtime, new tools). Bodies
must be scanned for vulnerabilities, pinned, and versioned — heavy, slow
processes. Roles must be editable by non-programmers — light, fast,
reviewable prose.

Twenty-two images would mean twenty-two vulnerability scans, twenty-two
pinned versions, and a roster change requiring a rebuild. Two images means
the roster changes by *editing text*, and the body layer changes only when
the body genuinely must.

So: **two harnesses, twenty-two roles.** A role is data; a harness is code.

## 7.2 Harness one: the general agent

The first executor is a self-contained general-agent container: a pinned
agent runtime, a curated tool set (file system, shell, network, web), and —
most importantly — the **hydration contract** described below. It suits the
majority of the roster: analysts, architects, designers, QA, compliance,
coordination — every role whose work product is analysis, decisions,
documents, and specifications.

Its work pattern, end to end:

1. Receive its work order (Chapter 4) as environment at boot.
2. **Hydrate**: clone the roster, resolve its role folder, load
   `persona.md`, `skill-and-tools.md`, `behavioral-spec.md`.
3. **Ground**: read what the organization knows (Chapter 9's memory) and
   the shared knowledge repository.
4. **Work**: perform the task per its role — research, analyze, write,
   compute, decide — within its turn budget.
5. **Write back**: record what it learned into shared memory.
6. **Report**: write the completion report and exit with a defined code.

## 7.3 Harness two: the code executor

The second executor serves the engineering roles — the workers whose
artifact is *a change to a real repository*: backend engineers, test
engineers, schema authors, security-control implementers.

Its pattern is the classic edit-ship loop, executed headlessly:

1. Clone the target project repository using its task-scoped credential.
2. Create a per-task branch (`task/<task_id>`) — the worker never touches a
   shared branch; isolation is per-task by construction.
3. Perform the edit under an agent coding harness, driven by the work
   order's instructions, with tool permissions explicitly granted — a
   permission configuration that had to be *learned* during the build
   (headless agents refuse to modify files unless the grant is explicit;
   Chapter 12 tells that story).
4. Commit with the task ID in the message, push the branch.
5. Report and exit.

The per-task branch is the concurrency model of the whole system: workers
never collide because workers never share a write surface. Parallel
execution strategies (worktrees, N-parallel) are a deliberate future item
(Chapter 13) — the sequential model was chosen for the first build because
correctness was cheaper to prove than concurrency.

## 7.4 The hydration contract

The moment a generic container becomes a specific specialist deserves
precision, because it is the hinge of the whole design. At boot, the worker:

1. clones the roster repository (a read that requires no secrets for a
   public roster — and a scoped token otherwise),
2. resolves the `#folder` half of its `agent_ref`,
3. reads the three role files in order — persona, skills-and-tools,
   behavioral spec — and *becomes* that role before any task work begins,
4. verifies its own readiness: a **self-report** stating which role it
   resolved, which persona files it loaded, and how long boot took.

The self-report is the worker's first act, and it exists for the same
reason Chapter 5's zero-survivors check exists: *claims must be verifiable.*
A worker that says "ready" without evidence is a worker the system cannot
trust; a worker that produces a structured boot record — role resolved,
spec received, three files loaded, milliseconds counted — can be gated on.

The measured boot cost of full hydration: ~0.8–1.7 seconds, including the
roster clone. That number is the reason the role/body separation costs
nothing operationally: re-staffing the organization is a text edit, and the
"hire" takes under two seconds.

## 7.5 Design decisions and alternatives

**Decision: hydration at boot, not baked at build.** Alternative: bake role
files into the image at build time. Rejected: that re-couples roles to
bodies (every roster edit = rebuild) and creates stale-role hazards. Boot
hydration keeps the roster authoritative *at the moment of work* — a worker
always plays the current version of its role.

**Decision: exit codes are the worker's vote; the report is its testimony.**
A worker exits 0 for success, non-zero for failure, and the report carries
the human-readable reason. The pipeline maps exit codes to outcomes
mechanically (Chapter 5) and never infers success from anything else — a
rule the build learned the hard way, when a cleanly-cleaned-up failed task
was briefly celebrated as a pass (Chapter 12).

**Decision: the harness holds no role knowledge.** The general harness has
no idea what a "risk analyst" is. It knows how to boot, hydrate, ground,
work, write back, and report. All identity lives in the roster. This is
what lets one harness serve thirty-one roles and a new role cost one text
file.

**Decision: two harnesses, honestly bounded.** The general harness's
boundary must be stated plainly: in the proof-of-concept build, its
"runtime" is the hydration-and-report chain — the deep autonomous
multi-tool loop was proven on the *code* harness. The general harness's
full in-loop autonomy is a known, named work item (Chapter 13), not a
hidden gap. A design document that hides its boundaries invites exactly
the paper-over failures Part III warns about.

## 7.6 What this buys

With two harnesses and a live roster, "hiring" is writing, "deployment" is
a container start, and "staffing a task" is one field in a work order. The
organization's headcount can triple in an afternoon without a single image
build — and every one of those workers arrives knowing its job, with a
clock already ticking, on a machine that will forget it ever existed.
