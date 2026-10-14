# Keep Worktrunk optional and require one branch per writer

- Date: 2026-07-30
- Paths: .agents/worktrunk, cmd/continuity/main.go, internal/continuity/coordination.go
- Related: commit 62261c7; recorded as D-009 in PROJECT_STATE.md until schema 3

## Decision

Keep Worktrunk as an optional orientation bridge and require one named branch per live writer.

Why: Worktrunk improves Git-worktree ergonomics, but shared context, task ownership, and safety must still work with plain Git and every harness.
