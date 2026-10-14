---
name: project-continuity
description: Resume, continue, or hand off non-trivial work in a repository with PROJECT_STATE.md, PROJECT_PLAN.md, and docs/adr. Use at session start, after context loss or compaction, before changing code, when the user asks why something is the way it is, and before ending a session.
---

# Project continuity

The repository's memory lives in three places. PROJECT_PLAN.md is the user's roadmap: Intent, Milestones, and Outcomes; the active milestone's open outcomes are the next work. docs/adr holds one decision per file. PROJECT_STATE.md indexes constraints, the code map, facts, context capsules, known pitfalls, and the last handoff.

## Resume

1. Read the session-start context, or run continuity context. It always shows index freshness, uncommitted files, broken decision records, stale facts, and constraints, then the next work, the last handoff, current decision titles, and capsules. Use --scope path to put matching decisions and capsules first.
2. If the index is stale, run continuity reconcile, update only the entries the listed changes affect, then run continuity sync.
3. If uncommitted changes exist, inspect them before editing; they can be unfinished work from the last session.

## While working

- Before changing an area, run continuity recall --path <file>. Claude Code and Codex hooks show linked decisions and pitfalls when a file is read or edited. For "why" questions or a topic, run continuity recall <words>.
- Follow current decisions. To change one, add a record that supersedes it with the record-decisions skill; a decision the user made changes only with the user's approval.
- When the user states or approves a decision, record it during the session with the record-decisions skill.
- Work only on open outcomes of the active milestone. Mark an outcome [x] after its acceptance is verified. Ask the user before adding or changing outcomes or milestones; with no active milestone, ask what comes next.
- Record a lasting failed approach under Known Pitfalls with its evidence paths in backticks. Add a Fact Card only for a fact later work relies on; a stale fact needs a review and continuity fact confirm.

## Hand off

Before ending, update the Handoff section: what was done, what is in progress, and the exact next step. Update the Code Map, capsules, pitfalls, and facts your work changed, run continuity sync, and commit. If no decision record was needed, say so.

Never store raw chats, reasoning, source code, secrets, or untrusted multiline text. Read references/state-schema.md and references/plan-schema.md for exact formats.
