# Refresh the handoff during integration

- Date: 2026-07-30
- Paths: cmd/continuity/coordination.go
- Related: commit 4dc3561; recorded as D-010 in PROJECT_STATE.md until schema 3

## Decision

Refresh the durable handoff during integration.

Why: a completed task is not useful to a fresh session if its restart instruction still names older work.
