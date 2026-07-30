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
5. Run `continuity ready` and select only an approved, unblocked `autonomous: yes` task with observable acceptance criteria. For planned work, use its outcome priority and direct unblocking evidence, then record the reason at claim time.

## Evidence gates

`PROJECT_STATE.md` may include compact source-linked Fact Cards and durable Blocker Cards. A stale fact or open blocker overlapping the task scope is an active gate, not background context: inspect the named evidence and use `continuity fact confirm` or `continuity blocker resolve` from an unclaimed integration checkout before retrying. Do not silently rewrite a fact's claim to make it fresh.

`CONTINUITY_POLICY.md` is an optional committed list of user-approved finalization commands. Add a command only after explicit user approval with `continuity policy add --approved ...`; relevant checks run automatically during `continuity finalize`. If one fails, inspect it and checkpoint the claim; do not create a completion or retry unchanged evidence. Read [governance-schema.md](references/governance-schema.md) before adding a fact, policy, or blocker.

## Intake and plan approval

If `PROJECT_PLAN.md` is a draft, inspect the repository and interview the user one decision at a time. Ask only for product choices that cannot be discovered locally, always give a recommendation, and capture compact facts rather than raw chat. Do not claim implementation work until the user explicitly approves the active milestone through `continuity plan approve --approved --milestone M-001`.

If the plan is approved, treat its active milestone and outcomes as the product boundary. Record product, priority, constraint, or milestone changes as plan deltas; do not silently extend scope. Read [plan-schema.md](references/plan-schema.md) before changing plan structure or approval state.

## Execute

Before editing, run `continuity start` with a stable harness owner ID. Planned tasks must link to an active approved outcome and carry a focused scope plus validation command; `start` defaults those fields from the task and requires `--selection-reason`. One worktree and one named branch have one writer; claims reject duplicate tasks, shared branches, and overlapping scopes in separate worktrees. From an unclaimed integration worktree, use `continuity task add --approved ...` for a user-specified task, or `continuity task add --derived --outcome O-001 --scope path --validates command --rationale text ...` for a needed technical subtask wholly inside an active approved outcome. The latter immediately commits the queue record; neither route authorizes new product work.

Honor the task contract. Checkpoint material progress, validation, or blockers with `continuity checkpoint`. Do not edit `PROJECT_STATE.md` as a live scratchpad; active progress belongs to the claim record.

## Complete and integrate

Run focused validation, commit the worker result, then finalize with the commit and factual evidence. Relevant approved policies run before the completion is recorded. A completion remains pending until its commit is present in the integration checkout. After merge or handoff, run `continuity integrate --id <task>` to update the durable index and unblock dependencies. Integration derives actual changed paths from the claim's Git range, ages affected fresh Fact Cards, writes `PROJECT_HISTORY/changes/<task>.md`, keeps at most three short path-scoped change capsules active, and gives the handoff the actual next ready task. Commit the state and generated archive together. Once every task linked to an active outcome is integrated, run `continuity plan outcome complete --id <outcome> --evidence 'verification: pass'`; milestone activation archives its completed outcome set.

Keep context bounded: use `continuity context --scope <task path>` first, then read only the matching Context Capsules, relevant state section, and mapped code. The durable index may hold approximately 30k tokens; routine startup must not load it wholesale. Read [state-schema.md](references/state-schema.md) when changing state structure or resolving ownership, freshness, or integration questions.

Worktrunk is optional. Its reviewed orientation hook template and parallel-worker workflow live in [../../worktrunk/README.md](../../worktrunk/README.md); plain Git remains the required baseline.
