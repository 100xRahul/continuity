# Build restart context by safety priority

- Date: 2026-08-03
- Paths: cmd/continuity/coordination.go, cmd/continuity/history.go, AGENTS.md, .agents/skills/project-continuity/SKILL.md
- Related: commit 9fcecce; recorded as D-015 in PROJECT_STATE.md until schema 3

## Decision

Compile restart context by safety priority and compact only archive-backed repetition during integration. Keep `AGENTS.md` as a discovery and invariant bootloader while the skill owns detailed procedure.

Why: a byte ceiling must never hide a gate or fabricate a startable task, and the active index must not become a second historical archive.
