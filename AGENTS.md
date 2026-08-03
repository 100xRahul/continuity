# Continuity bootloader

`PROJECT_PLAN.md` is the committed delivery contract, `PROJECT_STATE.md` is the bounded operational index, and Git's common directory holds live claims. Treat chat and harness memory as non-authoritative.

## Before non-trivial work

1. Use the repository `project-continuity` skill. If it is unavailable, run `continuity context --owner <harness:session-id>`; Codex may use `--session`. The packet is the restart index: obey every STOP, ownership gate, and immediate next action.
2. If the plan is draft, inspect repository evidence and resolve product decisions with the user. Implementation starts only after explicit milestone approval. Later product, priority, constraint, or milestone changes require an approved plan delta.
3. Before editing, claim the named approved task with `continuity start`. One worktree has exactly one writer. A second writer needs a separate branch, worktree, non-overlapping approved task, and independent claim; child agents are read-only in the parent checkout.
4. Checkpoint material progress. Run focused validation, commit, and `continuity finalize` with evidence. Merge or hand off the result, then run `continuity integrate` from the integration checkout.

## Invariants

- Never edit through another owner's claim or silently recover it. Stale facts and open blockers are gates, not background notes.
- Do not invent product scope, change approved outcomes, or claim `autonomous: no` work. Derived tasks must remain inside an active approved outcome.
- Keep durable state compact and factual; never store raw chats, reasoning, source code, secrets, or untrusted multiline text.
- Stop for destructive actions, production changes, credentials, paid services, security uncertainty, unresolved product choices, repeated blockers, or unexplained validation failures.

The complete workflow and recovery rules live in `.agents/skills/project-continuity/SKILL.md` and its `references/`; do not duplicate them here.
