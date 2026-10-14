# Derive change handoffs from the claimed Git range

- Date: 2026-07-30
- Paths: cmd/continuity/history.go, cmd/continuity/coordination.go, internal/continuity/git.go
- Related: commit 4a6f64b; recorded as D-013 in PROJECT_STATE.md until schema 3

## Decision

Derive compact change handoffs from the claimed Git range at integration time, preserve their full evidence in committed history, and retain only the latest three path-scoped change capsules in active state.

Why: later sessions can resume from factual intent, paths, validation, and next work without replaying code discovery, while old detail stays available without bloating routine context.
