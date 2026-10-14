# Give Codex bounded context, a live pulse, and write guards

- Date: 2026-07-30
- Paths: .codex/hooks.json, cmd/continuity/coordination.go
- Related: commit 667498e; recorded as D-007 in PROJECT_STATE.md until schema 3

## Decision

Codex hooks preload bounded context, emit a small live pulse on later prompts, and guard documented Bash and patch paths; child agents remain read-only in their parent checkout.

Why: this surfaces live ownership without repeated full-state stuffing, while Codex child hooks inherit the parent session ID and cannot safely distinguish simultaneous child writers.
