# Store live claims under Git's common directory

- Date: 2026-07-30
- Paths: internal/continuity/coordination.go
- Related: commit 667498e; recorded as D-005 in PROJECT_STATE.md until schema 3

## Decision

Store live claims, leases, and pending completions under Git's common directory.

Why: linked local worktrees share this local board without committing ephemeral progress or creating branch conflicts.
