# Treat facts, blockers, and approved policies as evidence gates

- Date: 2026-07-31
- Paths: cmd/continuity/governance.go, cmd/continuity/policy.go, internal/continuity/facts.go
- Related: commit c1102b2; recorded as D-014 in PROJECT_STATE.md until schema 3

## Decision

Treat source-linked facts, durable blockers, and explicitly user-approved policy checks as portable evidence gates.

Why: agents need a shared way to stop on changed assumptions or exhausted hypotheses without relying on a harness's private memory or silently expanding autonomy.
