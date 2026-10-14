# Keep the core dependency-free and harness-neutral

- Date: 2026-07-30
- Paths: cmd/continuity/main.go
- Related: commit 17ac164; recorded as D-002 in PROJECT_STATE.md until schema 3

## Decision

Keep the core dependency-free and harness-neutral.

Why: the CLI must work when hooks, skills, native memory, or MCP are unavailable.
