# Drop live coordination; parallel agents run in Orca

- Date: 2026-10-15
- Paths: cmd/continuity, internal/continuity, .codex/hooks.json, .agents/hooks
- Supersedes: 2026-07-30-live-claims-in-git-common-dir.md, 2026-07-30-one-writer-per-checkout.md, 2026-07-30-codex-context-pulse-and-guards.md, 2026-07-30-optional-worktrunk-one-branch-per-writer.md, 2026-07-30-refresh-handoff-on-integration.md, 2026-07-30-outcome-bound-task-contracts.md, 2026-07-30-change-handoffs-from-claimed-range.md, 2026-08-03-restart-context-by-safety-priority.md

## Context

- The user runs parallel agents in Orca, which gives each agent its own worktree, task list, ownership, and session history.
- The user's problem is lost context between sessions, not write collisions.

## Decision

- Remove claims, leases, heartbeats, the live claim store, start, checkpoint, finalize, integrate, status, ready, task, the Codex write guard and prompt pulse, and the Worktrunk bridge.
- The restart packet keeps a critical block (index freshness, uncommitted files, broken records, stale facts, constraints) and then shows the plan's next work, the handoff, decision titles, and capsules.
- The agent updates the Handoff section and runs continuity sync at the end of a session.

## Consequences

- Nothing stops two agents from writing one checkout; Orca's one worktree per agent covers that.
- About half of the Go code is gone, and the test suite runs in seconds.

## Reconsider when

- Parallel agents run without Orca or separate worktrees and collide in practice.
