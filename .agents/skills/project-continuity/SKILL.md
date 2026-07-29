---
name: project-continuity
description: Resume, perform, checkpoint, or finalize non-trivial engineering work in a repository containing PROJECT_STATE.md. Use at session start, after context loss or compaction, when changing harnesses, before code edits, and before handoff or completion.
---

# Project continuity

Use the repo-native state instead of reconstructing project context from chat history or broad code searches.

## Resume

1. Run `continuity brief` and read the emitted state.
2. If it is stale or the worktree is dirty, run `continuity reconcile`. Inspect only its listed changes and the code mapped to the active task.
3. Resume `Active Work`. If no work is active, run `continuity ready`; select only an explicit `autonomous: yes` task.
4. If the task has no approved goal, acceptance criteria, or safe boundary, stop and request user direction.

## Execute

Before editing, run `continuity start` unless the task is already active. Honor the task's execution mode:

- `strict`: follow the first action, edit scope, and validation unless the repository contradicts them.
- `soft`: begin with the mapped scope; allow narrow dependency discovery.
- `exploratory`: investigate the mapped area, but do not use broad repository orientation as a default.

Inspect code needed to verify current implementation. Do not copy code into `PROJECT_STATE.md`.

## Checkpoint and finalize

Run `continuity checkpoint` after material progress, a validation result, or a blocker. Include a factual summary, immediate restart action, and blocker if any.

Run `continuity finalize` only when the task's validation is complete and recorded. Then run `continuity check` before claiming success.

Keep only current, load-bearing facts in the state. Git is the archive. Read [state-schema.md](references/state-schema.md) when changing the state structure or resolving ownership/freshness questions.
