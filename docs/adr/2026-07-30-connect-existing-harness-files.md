# Connect existing harness files without overwriting them

- Date: 2026-07-30
- Paths: internal/continuity/adapters.go
- Related: commit 17ac164; recorded as D-004 in PROJECT_STATE.md until schema 3

## Decision

`continuity init` preserves existing instruction content, appends only a compact marked bridge when the shared protocol is absent, and semantically merges supported strict-JSON Codex/OpenCode adapters; unsupported structured files are preserved with an explicit warning and tool-owned skill paths remain non-overwriting.

Why: silently skipping an existing adapter makes Continuity undiscoverable in the exact mature projects that need it, while bounded text append and schema-aware JSON merge preserve user policy.
