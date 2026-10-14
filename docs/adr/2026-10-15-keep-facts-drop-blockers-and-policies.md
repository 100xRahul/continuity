# Keep source-linked facts; drop blockers and policy gates

- Date: 2026-10-15
- Paths: cmd/continuity/facts.go, internal/continuity/facts.go
- Supersedes: 2026-07-31-facts-blockers-policies-as-gates.md

## Decision

- Fact Cards stay: a fact becomes stale when its source paths change, the restart packet lists stale facts, and only continuity fact confirm with new evidence makes one fresh.
- Blocker Cards and CONTINUITY_POLICY.md checks are removed: they gated claims and finalize, which no longer exist. Failed approaches go in Known Pitfalls; checks belong in CI or harness hooks.

## Reconsider when

- Repeated failed attempts at the same problem recur across sessions despite recorded pitfalls.
