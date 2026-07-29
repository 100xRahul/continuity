# Continuity protocol

This repository uses `PROJECT_STATE.md` as the durable shared project state. Treat chat history and harness-specific memory as disposable conveniences, not authority.

## Before non-trivial work

1. Use the `project-continuity` skill when available; otherwise follow this file directly.
2. Run `continuity brief` and read `PROJECT_STATE.md` completely.
3. If the brief reports stale state or a dirty worktree, run `continuity reconcile`; inspect only the listed diff and mapped relevant code before updating state.
4. Resume `Active Work` when present. Otherwise, run `continuity ready` and continue only the first task marked `autonomous: yes` with explicit acceptance criteria.
5. Do not perform a broad repository orientation scan when the state is fresh.

## During work

- Before editing, run `continuity start` for a selected task unless it is already active.
- Follow the Active Work execution contract: mode, first action, edit scope, and validation.
- Checkpoint after a meaningful investigation, implementation, validation, or blocker transition.
- Keep claims factual. Record decisions with a short why and repository-relative evidence anchors. Record failures only when they remain relevant.

## Before handoff or completion

- Run focused validation required by the task.
- Use `continuity checkpoint` when work remains; use `continuity finalize` only after verification is recorded.
- Update decisions, code map, failures, verification, recent material changes, ready work, and handoff when affected.
- Run `continuity check` before claiming completion.

## Boundaries

- Do not change Product Intent, Approved Outcomes, or approval boundaries without explicit user direction.
- Do not invent product features or promote a task into the queue merely because it seems useful.
- Stop for destructive actions, production changes, credentials, external spending, security-sensitive uncertainty, unresolved product choices, repeated blockers, or unexplained failures.
