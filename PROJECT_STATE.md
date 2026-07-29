---
schema: 1
fingerprint: sha256:bb3055ca9871859d4c29e37377ae6c6799cf879a5cb822c0c36debc6b34718ce
branch: main
head: unborn
status: ready
active_task: none
updated_at: 2026-07-29T18:54:11Z
---

# Continuity

## Product Intent

Build a durable, cross-harness continuity system for AI coding agents.

## Approved Outcomes

- Deliver a dependency-free Go CLI that maintains and validates a compact Git-backed project state.
- Deliver one shared continuity protocol and skill with adapters for Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode.
- Prove recovery from fresh sessions, stale state, changed source, and task dependencies through automated tests and real CLI checks.

## Active Work
Task: none
Status: ready
Mode: strict
Goal: none
Acceptance criteria: none
Immediate next action: Commit the initial release, run continuity sync, and commit the resulting state metadata update.
Edit scope: none
Validation: none
## Ready Queue
- [x] T-001 | Implement the portable state engine and lifecycle CLI | depends: none | autonomous: yes | accepts: State parser, source fingerprinting, task readiness, lifecycle commands, and focused tests pass.
- [x] T-002 | Add shared instructions, skill, and harness adapters | depends: T-001 | autonomous: yes | accepts: Every target harness has a repository adapter pointing to the shared state and workflow.
- [x] T-003 | Verify state recovery, freshness, and task-graph behavior | depends: T-002 | autonomous: yes | accepts: Unit and command-level checks prove stale-state detection, recovery guidance, and task dependency behavior.
- [x] T-004 | Complete installation workflow, fresh-repository scaffolding, and documentation | depends: T-003 | autonomous: yes | accepts: Init scaffolds missing shared adapters without overwriting instructions, and README explains install, workflow, safety model, and adapter boundaries.
- [x] T-005 | Add post-commit metadata synchronization | depends: T-004 | autonomous: yes | accepts: A sync command records the current Git branch, HEAD, fingerprint, and timestamp without altering task state.
## Active Decisions

- D-001 — Keep all durable project knowledge in `PROJECT_STATE.md`; Git is the archive. Why: a bounded current projection avoids both vendor lock-in and transcript bloat. Evidence: `internal/continuity/state.go`.
- D-002 — Keep the core dependency-free and harness-neutral. Why: the CLI must work when hooks, skills, native memory, or MCP are unavailable. Evidence: `cmd/continuity/main.go`.
- D-003 — Treat native skills and hooks as adapters, not project authority. Why: harness support varies; the same repository state must remain portable. Evidence: `AGENTS.md`, `.agents/skills/project-continuity/`.
- D-004 — `continuity init` installs only missing protocol and adapter files. Why: existing repository instruction files are user-owned and cannot be safely merged or overwritten. Evidence: `internal/continuity/adapters.go`.

## Constraints and Approval Boundaries

- Do not invent product outcomes.
- Request approval for destructive actions, production changes, credentials, paid services, or unresolved product choices.
- Do not require one vendor's hooks, memory, plugin, MCP support, or cloud service for correct continuation.
- Keep the state current and bounded; do not store raw chats, reasoning traces, or pasted source code.

## Code Map

- CLI command routing — `cmd/continuity/main.go` — parses lifecycle commands and coordinates state/Git facts.
- State engine — `internal/continuity/state.go` — Markdown front matter, sections, validation, templates, and atomic writes.
- Git facts — `internal/continuity/git.go` — repository snapshots, source fingerprints, and incremental reconciliation.
- Task graph — `internal/continuity/tasks.go` — task syntax, dependencies, readiness, and cycle detection.
- Adapter installer — `internal/continuity/adapters.go` — non-overwriting protocol, skill, and harness adapter scaffolding for new repositories.
- Tests — `internal/continuity/*_test.go` — state round trip, fingerprint exclusions, task readiness, and cycles.

## Known Pitfalls and Failed Approaches

- Record only failures that remain relevant. Include the failed approach, evidence, resolution, and affected paths.
- Do not use an unanchored `.gitignore` entry named `continuity`; Git will ignore package directories with that name. Use `/continuity` for the root binary. Evidence: `.gitignore`.
- Exclude every generated harness adapter, including `opencode.json`, from the source fingerprint; otherwise a new initialized repository is stale immediately. Evidence: `internal/continuity/git.go`, `cmd/continuity/init_test.go`.

## Verification

- `go test ./...` — pass.
- `go test -race ./...` — pass.
- `go vet ./...` — pass.
- Official `quick_validate.py` for `.agents/skills/project-continuity` — pass.
- Fresh temporary-repository `continuity init` followed by `continuity check` — pass.
- `continuity sync` preserves active task state while refreshing Git metadata — pass.
- `continuity check` — pass for this state; uncommitted workspace changes are expected until the initial commit.

## Recent Material Changes
- Completed T-005: Added and verified post-commit metadata synchronization; the command refreshes branch, HEAD, fingerprint, and timestamp without altering task state.
- Completed T-004: Completed fresh-repository scaffolding, installation and cross-harness documentation, durable failure notes, and final release validation including unit, race, vet, skill, CLI, and fresh-init checks.
- Completed T-003: Verified lifecycle completion, dependency blocking, stale source rejection, Git-backed reconciliation, branch snapshots, and review-required evidence anchors through automated tests and a real stale-state run.
- Completed T-002: Added and tested shared repository adapters for Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode; all direct agents to the same project state and continuity workflow.
- Completed T-001: Implemented the Go continuity CLI with Markdown state validation, source fingerprints, Git snapshots, reconciliation output, acceptance-aware task dependencies, checkpoint/finalize lifecycle, and command-level tests.
- State initialized for Continuity.
- Added initial portable state engine, task graph, and unit tests.
- Added adapter scaffolding to `continuity init` with preservation checks for existing instructions.
## Handoff
Last completed: T-005 — Added and verified post-commit metadata synchronization; the command refreshes branch, HEAD, fingerprint, and timestamp without altering task state.
Immediate restart action: Commit the initial release, run continuity sync, and commit the resulting state metadata update.
