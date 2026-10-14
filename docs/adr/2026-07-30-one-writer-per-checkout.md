# Allow one writer per checkout

- Date: 2026-07-30
- Paths: cmd/continuity/coordination.go, AGENTS.md
- Related: commit 667498e; recorded as D-006 in PROJECT_STATE.md until schema 3

## Decision

One checkout has one writer; parallel writers require separate worktrees, approved tasks, and non-overlapping scopes.

Why: independent agents cannot safely infer whether apparent unrelated changes share hidden assumptions.
