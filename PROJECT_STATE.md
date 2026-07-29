---
schema: 1
fingerprint: sha256:2d5365dfa715dcef10f2974037a039ae68ffe71b245b43c3ee9a84d1083673fd
branch: main
head: 667498ef75cbec10d4a2caeb99b0f6d722f2cc82
status: ready
active_task: none
updated_at: 2026-07-29T19:53:52Z
---

# Continuity

## Product Intent

Build a durable, cross-harness continuity system for AI coding agents.

## Approved Outcomes

- Deliver a dependency-free Go CLI that maintains and validates a compact Git-backed project state.
- Deliver one shared continuity protocol and skill with adapters for Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode.
- Prove recovery from fresh sessions, stale state, changed source, and task dependencies through automated tests and real CLI checks.
- Deliver safe same-machine multi-session coordination: agents must see live task claims before editing, and separate worktrees must not duplicate or silently conflict on work.

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
## Active Decisions

- D-001 — Keep committed durable project knowledge in `PROJECT_STATE.md`; Git is the archive. Keep short-lived same-machine ownership outside that file. Why: a bounded current projection avoids both vendor lock-in and transcript bloat without turning progress pings into merge conflicts. Evidence: `internal/continuity/state.go`, `internal/continuity/coordination.go`.
- D-002 — Keep the core dependency-free and harness-neutral. Why: the CLI must work when hooks, skills, native memory, or MCP are unavailable. Evidence: `cmd/continuity/main.go`.
- D-003 — Treat native skills and hooks as adapters, not project authority. Why: harness support varies; the same repository state must remain portable. Evidence: `AGENTS.md`, `.agents/skills/project-continuity/`.
- D-004 — `continuity init` installs only missing protocol and adapter files. Why: existing repository instruction files are user-owned and cannot be safely merged or overwritten. Evidence: `internal/continuity/adapters.go`.
- D-005 — Store live claims, leases, and pending completions under Git's common directory. Why: linked local worktrees share this local board without committing ephemeral progress or creating branch conflicts. Evidence: `internal/continuity/coordination.go`.
- D-006 — One checkout has one writer; parallel writers require separate worktrees, approved tasks, and non-overlapping scopes. Why: independent agents cannot safely infer whether apparent unrelated changes share hidden assumptions. Evidence: `cmd/continuity/coordination.go`, `AGENTS.md`.
- D-007 — Codex hooks preload bounded context, emit a small live pulse on later prompts, and guard documented Bash and patch paths; child agents remain read-only in their parent checkout. Why: this surfaces live ownership without repeated full-state stuffing, while Codex child hooks inherit the parent session ID and cannot safely distinguish simultaneous child writers. Evidence: `.codex/hooks.json`, `cmd/continuity/coordination.go`.

## Constraints and Approval Boundaries

- Do not invent product outcomes.
- Request approval for destructive actions, production changes, credentials, paid services, or unresolved product choices.
- Do not require one vendor's hooks, memory, plugin, MCP support, or cloud service for correct continuation.
- Keep the state current and bounded; do not store raw chats, reasoning traces, or pasted source code.

## Code Map

- CLI command routing — `cmd/continuity/main.go` — parses lifecycle commands and coordinates state/Git facts.
- Live coordination commands — `cmd/continuity/coordination.go` — bounded context packets, claims, lifecycle integration, and Codex hook output.
- State engine — `internal/continuity/state.go` — Markdown front matter, sections, validation, templates, and atomic writes.
- Git facts — `internal/continuity/git.go` — repository snapshots, source fingerprints, and incremental reconciliation.
- Coordination store — `internal/continuity/coordination.go` — common-Git-directory claims, heartbeats, scope exclusion, pending completions, and explicit stale recovery.
- Task graph — `internal/continuity/tasks.go` — task syntax, dependencies, readiness, and cycle detection.
- Adapter installer — `internal/continuity/adapters.go` — non-overwriting protocol, skill, and harness adapter scaffolding for new repositories.
- Codex adapter — `.codex/hooks.json`, `.codex/hooks/continuity.sh` — optional session context and pre-write guardrails after project trust.
- Tests — `internal/continuity/*_test.go` — state round trip, fingerprint exclusions, task readiness, and cycles.

## Known Pitfalls and Failed Approaches

- Record only failures that remain relevant. Include the failed approach, evidence, resolution, and affected paths.
- Do not use an unanchored `.gitignore` entry named `continuity`; Git will ignore package directories with that name. Use `/continuity` for the root binary. Evidence: `.gitignore`.
- Exclude every generated harness adapter, including `opencode.json`, from the source fingerprint; otherwise a new initialized repository is stale immediately. Evidence: `internal/continuity/git.go`, `cmd/continuity/init_test.go`.
- Do not use a fixed `PROJECT_STATE.md.tmp` name for atomic writes. Concurrent processes can rename or remove each other's temporary file; use a unique same-directory temporary file and rename it atomically. Evidence: `internal/continuity/state.go`, `internal/continuity/coordination_test.go`.
- Do not walk linked-worktree `.git` metadata when fingerprinting. Its pointer file contains worktree-specific paths and creates false staleness; derive files through `git ls-files` instead. Evidence: `internal/continuity/git.go`, `internal/continuity/coordination_test.go`.
- Do not place unchecked multiline lifecycle text in Markdown state. It can forge a second heading or queue entry; claims, checkpoints, and completions accept only one-line fields. Evidence: `cmd/continuity/main.go`, `internal/continuity/coordination.go`.
- Do not let multiple Codex child agents write one parent checkout. They share the parent's hook session ID; use separate worktrees for independent writers. Evidence: `AGENTS.md`, `.codex/hooks.json`.

## Verification
- go vet ./... and git diff --check: pass
- go test -race ./... -count=1: pass
- go test ./... -count=1: pass
- `sh -n .codex/hooks/continuity.sh` and Ruby JSON parsing of `.codex/hooks.json` — pass.
- Simulated Codex `SessionStart`, `SubagentStart`, user-prompt pulse, Bash, and `apply_patch` hook inputs — pass; non-owner writes are denied and read-only inspection remains available.
- Fresh temporary-repository `continuity init` followed by `continuity check` — pass.
- `continuity sync` preserves active task state while refreshing Git metadata — pass.
## Recent Material Changes
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
Last completed: T-006 — Implemented bounded same-machine multi-session coordination with exclusive claims, safe lifecycle integration, prompt pulses, and Codex write guards.
Immediate restart action: Run continuity context and continuity status. No autonomous work remains; record and approve the next outcome before claiming new work.
