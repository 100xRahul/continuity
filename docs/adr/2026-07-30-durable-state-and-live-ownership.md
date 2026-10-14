# Keep durable knowledge in PROJECT_STATE.md and live ownership outside it

- Date: 2026-07-30
- Paths: internal/continuity/state.go, internal/continuity/coordination.go
- Related: commit 17ac164; recorded as D-001 in PROJECT_STATE.md until schema 3

## Decision

Keep committed durable project knowledge in `PROJECT_STATE.md`; Git is the archive. Keep short-lived same-machine ownership outside that file.

Why: a bounded current projection avoids both vendor lock-in and transcript bloat without turning progress pings into merge conflicts.
