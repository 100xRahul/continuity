# Allow about 30k tokens of durable state, selected by path

- Date: 2026-07-30
- Paths: internal/continuity/state.go, cmd/continuity/context.go
- Related: commit 259d2a2; recorded as D-008 in PROJECT_STATE.md until schema 3

## Decision

Permit approximately 30,000 tokens of durable project knowledge, but select context through path-indexed capsules.

Why: larger projects need richer accumulated facts, while routine session startup must remain small and relevant.
