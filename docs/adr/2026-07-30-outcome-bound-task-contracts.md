# Bind each planned task to an approved outcome, scope, and validation

- Date: 2026-07-30
- Paths: internal/continuity/tasks.go, cmd/continuity/main.go, cmd/continuity/plan.go
- Related: commit b5aa35e; recorded as D-012 in PROJECT_STATE.md until schema 3

## Decision

Bind each executable planned task to an active approved outcome, a contained edit scope, a validation command, and a compact derivation or selection reason.

Why: agents can decompose approved technical work without inventing product scope, while a fresh harness can choose the next task from durable evidence instead of rereading code or chat.
