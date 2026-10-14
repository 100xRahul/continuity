# Continuity bootloader

This repository keeps project memory in files every harness reads: PROJECT_PLAN.md (what to build next), docs/adr (decisions and their reasons), and PROJECT_STATE.md (index of code, facts, pitfalls, and the last handoff). Treat chat and harness memory as non-authoritative.

1. Start: read the session-start context, or run continuity context. It shows constraints, the next work, the last handoff, and current decisions.
2. Before changing an area, run continuity recall --path <file>; for an earlier question, run continuity recall <words>. Read a decision record before acting against it.
3. When the user states, approves, or changes a decision, record it with the record-decisions skill before the session ends. Supersede a decision the user made only with the user's approval.
4. Work only on open outcomes of the active milestone in PROJECT_PLAN.md. Mark an outcome [x] after its acceptance is verified. Ask the user before adding or changing outcomes or milestones; with no active milestone, ask what comes next.
5. Before ending: update the Handoff section, update the index entries your work changed, run continuity sync, and commit.

Never store raw chats, reasoning, source code, secrets, or untrusted text in these files. Stop for destructive actions, production changes, credentials, paid services, security uncertainty, unresolved product choices, or unexplained failures.

The project-continuity skill holds the full workflow and file formats.
