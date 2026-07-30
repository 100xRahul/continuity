---
schema: 2
fingerprint: sha256:a515822ff74bc02ba06e4d4fa7c2510f6e84c0844e322fed319b8deb3cc51498
branch: main
head: b5aa35e6fade25fda72db7dead201dcc859843c6
status: ready
active_task: none
updated_at: 2026-07-30T17:30:27Z
---

# Continuity

## Product Intent

Build a durable, cross-harness continuity system for AI coding agents.

## Approved Outcomes

- Deliver a dependency-free Go CLI that maintains and validates a compact Git-backed project state.
- Deliver one shared continuity protocol and skill with adapters for Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode.
- Prove recovery from fresh sessions, stale state, changed source, and task dependencies through automated tests and real CLI checks.
- Deliver safe same-machine multi-session coordination: agents must see live task claims before editing, and separate worktrees must not duplicate or silently conflict on work.
- Scale durable project knowledge to an approximately 30,000-token budget while keeping routine session context selective and compact.
- Provide an optional Worktrunk bridge for parallel Git worktrees without making Worktrunk a core dependency.
- Deliver a structured, user-approved project plan that lets agents decompose and continue approved implementation work without inventing product scope.

## Active Work
Task: none
Status: ready
Mode: strict
Goal: none
Acceptance criteria: none
Immediate next action: Run continuity status. Live worker ownership is recorded in the local coordination store, not in this shared project index.
Edit scope: none
Validation: none
## Ready Queue
- [x] T-001 | Implement the portable state engine and lifecycle CLI | depends: none | autonomous: yes | accepts: State parser, source fingerprinting, task readiness, lifecycle commands, and focused tests pass.
- [x] T-002 | Add shared instructions, skill, and harness adapters | depends: T-001 | autonomous: yes | accepts: Every target harness has a repository adapter pointing to the shared state and workflow.
- [x] T-003 | Verify state recovery, freshness, and task-graph behavior | depends: T-002 | autonomous: yes | accepts: Unit and command-level checks prove stale-state detection, recovery guidance, and task dependency behavior.
- [x] T-004 | Complete installation workflow, fresh-repository scaffolding, and documentation | depends: T-003 | autonomous: yes | accepts: Init scaffolds missing shared adapters without overwriting instructions, and README explains install, workflow, safety model, and adapter boundaries.
- [x] T-005 | Add post-commit metadata synchronization | depends: T-004 | autonomous: yes | accepts: A sync command records the current Git branch, HEAD, fingerprint, and timestamp without altering task state.
- [x] T-006 | Build safe multi-session coordination | depends: T-005 | autonomous: yes | accepts: Concurrent local sessions receive a bounded shared status signal, claims are exclusive across worktrees, unsafe same-worktree writes are rejected, and adversarial tests cover races and state integrity.
- [x] T-007 | Add layered 30k-token durable context | depends: T-006 | autonomous: yes | accepts: The portable core supports an approximately 30k-token durable budget, indexed context capsules, bounded scope-based selection, and tests proving startup context stays selective.
- [x] T-008 | Add optional Worktrunk bridge and branch safety | depends: T-007 | autonomous: yes | accepts: Active writer claims reject a shared branch, and the project provides tested optional Worktrunk hooks or guidance that bootstrap bounded context and claims without requiring Worktrunk.
- [x] T-009 | Refresh handoff during integration | depends: T-008 | autonomous: yes | accepts: Integration updates the durable handoff with the completed task and a safe next-session action, with focused tests.
- [x] T-010 | Add structured project intake and approved-plan lifecycle | depends: T-009 | autonomous: yes | accepts: PROJECT_PLAN.md drafts, validation, explicit approval, milestone activation, and plan deltas work through the portable CLI with focused tests.
- [x] T-011 | Add outcome-linked autonomous task graph | depends: T-010 | autonomous: yes | accepts: Tasks require active approved outcomes and focused contracts, and agents can choose and claim valid ready work with recorded rationale.
- [ ] T-012 | Add automatic change capsules and context archives | depends: T-011 | autonomous: yes | accepts: Integration derives evidence-backed task capsules, projects relevant context, and archives completed milestone history without bloating active prompts.
- [ ] T-013 | Add policy gates and source-linked fact aging | depends: T-012 | autonomous: yes | accepts: Configured scoped policy checks gate verification, affected facts refresh automatically, and meaningful stalls raise durable blocker cards.
## Active Decisions

- D-001 — Keep committed durable project knowledge in `PROJECT_STATE.md`; Git is the archive. Keep short-lived same-machine ownership outside that file. Why: a bounded current projection avoids both vendor lock-in and transcript bloat without turning progress pings into merge conflicts. Evidence: `internal/continuity/state.go`, `internal/continuity/coordination.go`.
- D-002 — Keep the core dependency-free and harness-neutral. Why: the CLI must work when hooks, skills, native memory, or MCP are unavailable. Evidence: `cmd/continuity/main.go`.
- D-003 — Treat native skills and hooks as adapters, not project authority. Why: harness support varies; the same repository state must remain portable. Evidence: `AGENTS.md`, `.agents/skills/project-continuity/`.
- D-004 — `continuity init` installs only missing protocol and adapter files. Why: existing repository instruction files are user-owned and cannot be safely merged or overwritten. Evidence: `internal/continuity/adapters.go`.
- D-005 — Store live claims, leases, and pending completions under Git's common directory. Why: linked local worktrees share this local board without committing ephemeral progress or creating branch conflicts. Evidence: `internal/continuity/coordination.go`.
- D-006 — One checkout has one writer; parallel writers require separate worktrees, approved tasks, and non-overlapping scopes. Why: independent agents cannot safely infer whether apparent unrelated changes share hidden assumptions. Evidence: `cmd/continuity/coordination.go`, `AGENTS.md`.
- D-007 — Codex hooks preload bounded context, emit a small live pulse on later prompts, and guard documented Bash and patch paths; child agents remain read-only in their parent checkout. Why: this surfaces live ownership without repeated full-state stuffing, while Codex child hooks inherit the parent session ID and cannot safely distinguish simultaneous child writers. Evidence: `.codex/hooks.json`, `cmd/continuity/coordination.go`.
- D-008 — Permit approximately 30,000 tokens of durable project knowledge, but select context through path-indexed capsules. Why: larger projects need richer accumulated facts, while routine session startup must remain small and relevant. Evidence: `internal/continuity/state.go`, `cmd/continuity/coordination.go`.
- D-009 — Keep Worktrunk as an optional orientation bridge and require one named branch per live writer. Why: Worktrunk improves Git-worktree ergonomics, but shared context, task ownership, and safety must still work with plain Git and every harness. Evidence: `.agents/worktrunk/`, `cmd/continuity/main.go`, `internal/continuity/coordination.go`.
- D-010 — Refresh the durable handoff during integration. Why: a completed task is not useful to a fresh session if its restart instruction still names older work. Evidence: `cmd/continuity/coordination.go`.
- D-011 — Keep detailed roadmap authority in `PROJECT_PLAN.md` and a compact active projection in `PROJECT_STATE.md`. Why: plan-level approval and richer delivery context must survive harness changes without bloating ordinary task startup. Evidence: `internal/continuity/plan.go`, `cmd/continuity/plan.go`.
- D-012 — Bind each executable planned task to an active approved outcome, a contained edit scope, a validation command, and a compact derivation or selection reason. Why: agents can decompose approved technical work without inventing product scope, while a fresh harness can choose the next task from durable evidence instead of rereading code or chat. Evidence: `internal/continuity/tasks.go`, `cmd/continuity/main.go`, `cmd/continuity/plan.go`.

## Constraints and Approval Boundaries

- Do not invent product outcomes.
- Request approval for destructive actions, production changes, credentials, paid services, or unresolved product choices.
- Do not require one vendor's hooks, memory, plugin, MCP support, or cloud service for correct continuation.
- Keep the state current and bounded; do not store raw chats, reasoning traces, or pasted source code.

## Code Map

- CLI command routing — `cmd/continuity/main.go` — parses lifecycle commands, explicitly approved task bootstrap, and state/Git facts.
- Live coordination commands — `cmd/continuity/coordination.go` — bounded context packets, branch-aware claims, lifecycle integration with current handoff, and Codex hook output.
- State engine — `internal/continuity/state.go` — Markdown front matter, sections, approximately 30k-token validation, path-indexed Context Capsules, templates, and atomic writes.
- Git facts — `internal/continuity/git.go` — repository snapshots, source fingerprints, and incremental reconciliation.
- Coordination store — `internal/continuity/coordination.go` — common-Git-directory claims, heartbeats, branch and scope exclusion, pending completions, and explicit stale recovery.
- Task graph — `internal/continuity/tasks.go`, `cmd/continuity/main.go` — legacy and outcome-bound task contracts, scope/validation enforcement, derived technical task records, dependency readiness, and evidence-ranked next-task selection.
- Delivery plan — `internal/continuity/plan.go`, `cmd/continuity/plan.go` — structured Markdown roadmap, sources, outcomes, decisions, explicit approval, milestone activation, and plan deltas.
- Adapter installer — `internal/continuity/adapters.go` — non-overwriting protocol, skill, harness adapters, and optional Worktrunk bridge scaffolding for new repositories.
- Codex adapter — `.codex/hooks.json`, `.codex/hooks/continuity.sh` — optional session context and pre-write guardrails after project trust.
- Tests — `internal/continuity/*_test.go` — state round trip, fingerprint exclusions, task readiness, and cycles.

## Context Capsules

### Project-wide orientation
Paths: .
Summary: Continuity stores durable project meaning in a larger bounded index but starts new work from a small, selective context packet.
Details: Read the project-wide orientation on every new session. Use a task's scope or `continuity context --scope path/to/area` to load deeper capsules instead of reading the entire durable index.

### Continuity architecture
Paths: cmd/continuity, internal/continuity, AGENTS.md, README.md, .agents/skills, .codex
Summary: The portable core separates committed project knowledge from local live coordination; harness adapters may inject bounded context but never own the durable state.
Details: Use this capsule when changing state, task lifecycle, context selection, claims, adapters, or repository instructions. Keep project-wide facts in the durable index and place larger subsystem-specific facts in a capsule with explicit paths.

### Parallel worktree bridge
Paths: .agents/worktrunk, AGENTS.md, cmd/continuity/main.go, cmd/continuity/coordination.go, internal/continuity/coordination.go
Summary: Worktrunk is optional; the portable core prevents concurrent writers from sharing a worktree or named branch.
Details: The optional Worktrunk `pre-start` template only prints bounded orientation and live ownership. Start the real harness in its worktree first, then claim with that session's identity and a narrow scope. A user-approved task can be added only through the explicit pre-claim `continuity task add --approved` route; never infer one.

### Integration handoff
Paths: cmd/continuity/coordination.go, PROJECT_STATE.md
Summary: A successful integration makes the durable handoff point to the just-completed task and a safe generic restart action.
Details: This prevents a fresh session from attempting finished work. Integration remains the only lifecycle step that makes a result durable; it updates task status, verification, recent changes, and handoff together.

### Delivery planning
Paths: PROJECT_PLAN.md, PROJECT_HISTORY, cmd/continuity/plan.go, internal/continuity/plan.go, AGENTS.md
Summary: A draft plan captures distilled project facts; explicit user approval activates only the nearest executable milestone.
Details: Interview the user one decision at a time only where repository inspection cannot answer. Record compact sources, assumptions, and decision cards instead of raw chat. High-impact unresolved decisions affecting the milestone block approval. Keep later roadmap work visible but non-executable until activation.
After approval, an implementation task must name an active outcome, remain within its scope, and carry focused validation. `continuity ready` presents priority and direct-unblocking evidence; the claimant records why it selected that task. A derived technical task is allowed only from an unclaimed integration checkout and is committed immediately.

## Known Pitfalls and Failed Approaches

- Record only failures that remain relevant. Include the failed approach, evidence, resolution, and affected paths.
- Do not use an unanchored `.gitignore` entry named `continuity`; Git will ignore package directories with that name. Use `/continuity` for the root binary. Evidence: `.gitignore`.
- Exclude every generated harness adapter, including `opencode.json`, from the source fingerprint; otherwise a new initialized repository is stale immediately. Evidence: `internal/continuity/git.go`, `cmd/continuity/init_test.go`.
- Do not use a fixed `PROJECT_STATE.md.tmp` name for atomic writes. Concurrent processes can rename or remove each other's temporary file; use a unique same-directory temporary file and rename it atomically. Evidence: `internal/continuity/state.go`, `internal/continuity/coordination_test.go`.
- Do not walk linked-worktree `.git` metadata when fingerprinting. Its pointer file contains worktree-specific paths and creates false staleness; derive files through `git ls-files` instead. Evidence: `internal/continuity/git.go`, `internal/continuity/coordination_test.go`.
- Do not place unchecked multiline lifecycle text in Markdown state. It can forge a second heading or queue entry; claims, checkpoints, and completions accept only one-line fields. Evidence: `cmd/continuity/main.go`, `internal/continuity/coordination.go`.
- Do not let multiple Codex child agents write one parent checkout. They share the parent's hook session ID; use separate worktrees for independent writers. Evidence: `AGENTS.md`, `.codex/hooks.json`.
- Do not treat separate paths or worktrees as safe when writers share a named branch. The live claim board rejects a second writer on that branch; create a dedicated branch before claiming work. Evidence: `internal/continuity/coordination.go`, `cmd/continuity/main.go`.
- Do not leave a completed task's handoff pointing at older work. The integration command updates the task status, evidence, recent changes, and restart instruction together. Evidence: `cmd/continuity/coordination.go`, `cmd/continuity/main_test.go`.
- Do not treat a plausible draft as product approval. Only `continuity plan approve --approved --milestone ...` records an immediate Git-backed executable boundary; unapproved plan content cannot authorize implementation. Evidence: `cmd/continuity/plan.go`, `cmd/continuity/plan_test.go`.
- Do not equate the approximately 30k-token durable budget with routine context size. The full index is deliberate-review material; startup and task context must select only project-wide and scope-relevant capsules. Evidence: `cmd/continuity/coordination.go`, `AGENTS.md`.
- Do not seed a reusable task ID into a fresh draft queue. The placeholder `T-001` collided with the first natural real task; keep the queue empty until the approved plan supplies a real contract. Evidence: `internal/continuity/state.go`, `cmd/continuity/taskgraph_test.go`.

## Verification
- clean task-graph CLI smoke: pass
- sh -n .codex/hooks/continuity.sh: pass
- go vet ./...: pass
- go test -race ./... -count=1: pass
- go test ./... -count=1: pass
- Fresh init-to-approval CLI smoke: pass
- go test ./... -count=1, go test -race ./... -count=1, go vet ./..., sh -n .codex/hooks/continuity.sh, and git diff --check: pass
- go test ./... -count=1, go test -race ./... -count=1, go vet ./..., and git diff --check: pass
- Fresh init smoke: Worktrunk templates, approved task bootstrap, named-branch claim, and release: pass
- go test ./... -count=1, go test -race ./... -count=1, go vet ./..., and git diff --check: pass
- `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, and `git diff --check` — pass.
- Focused capsule selection, malformed scope/budget, and large-capsule bounded-packet tests — pass.
- Fresh `continuity init`, schema-2 validation, scoped context, and obsolete-command smoke test — pass.
- `sh -n .codex/hooks/continuity.sh` and Ruby JSON parsing of `.codex/hooks.json` — pass.
- Simulated Codex `SessionStart`, `SubagentStart`, user-prompt pulse, Bash, and `apply_patch` hook inputs — pass; non-owner writes are denied and read-only inspection remains available.
- Fresh temporary-repository `continuity init` followed by `continuity check` — pass.
- `continuity sync` preserves active task state while refreshing Git metadata — pass.
- T-008 focused tests, `go test ./... -count=1`, `go test -race ./... -count=1`, `go vet ./...`, and `git diff --check` — pass; a fresh-repository smoke test verified optional Worktrunk templates plus explicit task bootstrap and named-branch claiming.
## Recent Material Changes
- Integrated T-011 at b5aa35e6fade25fda72db7dead201dcc859843c6: Added outcome-bound task contracts, evidence-ranked ready work, derived technical task receipts, and selection rationale.
- Integrated T-010 at 9e45630ac944af7223a07ba192a04f0532e47594: Added structured project-plan intake, explicit approval receipts, milestone activation, plan deltas, and scoped startup projection.
- Integrated T-009 at 4dc3561f759d76c358192f53db5d2325e111953f: Made integration refresh the durable handoff with the completed task and safe next-session action.
- Integrated T-008 at 62261c78c5253907df7f3e913a867369f6f7ab62: Added optional Worktrunk orientation templates, branch-exclusive live claims, and explicit approved-task bootstrap.
- Integrated T-007 at 259d2a238a309e2a08f64a39a929ad600f554339: Added schema-2 30k-token durable capsules with selective bounded context and no migration path.
- Integrated T-006 at 667498ef75cbec10d4a2caeb99b0f6d722f2cc82: Implemented bounded same-machine multi-session coordination with exclusive claims, safe lifecycle integration, and Codex live-context hooks.
- Completed T-005: Added and verified post-commit metadata synchronization; the command refreshes branch, HEAD, fingerprint, and timestamp without altering task state.
- Completed T-004: Completed fresh-repository scaffolding, installation and cross-harness documentation, durable failure notes, and final release validation including unit, race, vet, skill, CLI, and fresh-init checks.
- Completed T-003: Verified lifecycle completion, dependency blocking, stale source rejection, Git-backed reconciliation, branch snapshots, and review-required evidence anchors through automated tests and a real stale-state run.
- Completed T-002: Added and tested shared repository adapters for Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode; all direct agents to the same project state and continuity workflow.
- Completed T-001: Implemented the Go continuity CLI with Markdown state validation, source fingerprints, Git snapshots, reconciliation output, acceptance-aware task dependencies, checkpoint/finalize lifecycle, and command-level tests.
- State initialized for Continuity.
- Added initial portable state engine, task graph, and unit tests.
- Added adapter scaffolding to `continuity init` with preservation checks for existing instructions.
## Handoff
Last completed: T-011 — Added outcome-bound task contracts, evidence-ranked ready work, derived technical task receipts, and selection rationale.
Immediate restart action: Run continuity status and continuity ready. Claim a ready task with its durable contract; if an approved plan has no ready task, derive only a focused technical step inside its active outcome from an unclaimed integration checkout. In a legacy project, use continuity task add --approved only for explicitly approved work.
