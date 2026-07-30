---
name: project-continuity
description: Resume, coordinate, perform, checkpoint, finalize, or integrate non-trivial engineering work in a repository containing PROJECT_STATE.md. Use at session start, after context loss or compaction, when another local agent session may be active, before code edits, and before handoff or completion.
---

# Project continuity

Use the committed project index for durable meaning and the live local claim board for concurrent ownership. Do not reconstruct either from chat history or broad code searches.

## Resume

1. Use automatic harness context when available; otherwise run `continuity context --owner <harness:session-id>` and `continuity status`. Codex may use `--session` as a convenience alias.
2. If another owner holds this worktree, do not edit or reinterpret its changes. Work read-only, wait, or use a separate worktree.
3. A Codex child agent is read-only in its parent worktree; use a separate worktree and claim for a child writer.
4. If state is stale, run `continuity reconcile`; inspect only listed changes and mapped code. Update durable facts and run `continuity sync` before claiming a task.
5. Select only an approved, unblocked `autonomous: yes` task with observable acceptance criteria.

## Execute

Before editing, run `continuity start` with a stable harness owner ID and a narrow repository-relative `--scope`. One worktree and one named branch have one writer; claims reject duplicate tasks, shared branches, and overlapping scopes in separate worktrees. If a user has approved a new task before any claim exists, use `continuity task add --approved ...` from the unclaimed integration worktree; never invent product work.

Honor the task contract. Checkpoint material progress, validation, or blockers with `continuity checkpoint`. Do not edit `PROJECT_STATE.md` as a live scratchpad; active progress belongs to the claim record.

## Complete and integrate

Run focused validation, commit the worker result, then finalize with the commit and factual evidence. A completion remains pending until its commit is present in the integration checkout. After merge or handoff, run `continuity integrate --id <task>` to update the durable index and unblock dependencies.

Keep context bounded: use `continuity context --scope <task path>` first, then read only the matching Context Capsules, relevant state section, and mapped code. The durable index may hold approximately 30k tokens; routine startup must not load it wholesale. Read [state-schema.md](references/state-schema.md) when changing state structure or resolving ownership, freshness, or integration questions.

Worktrunk is optional. Its reviewed orientation hook template and parallel-worker workflow live in [../../worktrunk/README.md](../../worktrunk/README.md); plain Git remains the required baseline.
