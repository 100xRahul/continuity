# Continuity protocol

This repository has two complementary memories:

- `PROJECT_STATE.md` is the compact, committed project index: intent, approvals, decisions, code map, durable failures, and integrated work.
- Git's local common directory holds short-lived shared coordination: live claims, heartbeats, and completed results awaiting integration. It is shared by local Git worktrees but is never committed or used as a chat transcript.

Treat harness memory and chat history as disposable conveniences, not authority.

## Start or resume a session

1. Use the `project-continuity` skill when available. In Codex, trust this repository's optional hooks once with `/hooks`; a trusted SessionStart hook injects a bounded live coordination snapshot automatically.
2. Otherwise run `continuity context --session <Codex-session-id>` and `continuity status`. Read `PROJECT_STATE.md` only when the context packet points to a relevant section.
3. If another owner has a claim in this worktree, do not edit, revert, reinterpret, or “clean up” its changes. Work read-only, wait, or open a separate Git worktree.
4. If state is stale, run `continuity reconcile`, inspect only the listed/mapped evidence, update durable facts, and run `continuity sync` before claiming new work.

## Claim before writing

- One worktree has exactly one writer. A second writer in the same checkout is unsafe even when it thinks its task is unrelated.
- Codex child agents in one parent session are read-only in that parent checkout. Their hook identity is the parent session, so a child that needs to write must receive a separate Git worktree and an independent claim.
- Before any code edit, claim one explicit autonomous task: `continuity start --id T-001 --session <Codex-session-id> ... --scope path/to/area`.
- A claim is exclusive by task ID, worktree, and overlapping path scope. An omitted scope means the whole repository and intentionally prevents parallel work.
- Different writers require different worktrees, different approved tasks, and non-overlapping scopes. They must not share a branch or directly edit the committed project index while working.
- Use `continuity heartbeat` or a checkpoint after material progress. Release only your own claim; recover a stale claim explicitly after inspecting `continuity status`.

## Complete safely

1. Run the task's focused validation and commit the worker result.
2. Run `continuity finalize --commit <commit> --summary ... --evidence 'command: result'`. This creates a local completion record; it does not falsely mark the shared project index complete.
3. Merge or hand off the commit into the integration checkout, then run `continuity integrate --id T-001`. Only integration updates `PROJECT_STATE.md` and unblocks dependent work.

## Boundaries

- Do not change Product Intent, Approved Outcomes, priorities, or approval boundaries without explicit user direction.
- Do not claim `autonomous: no` work. Do not invent tasks or product features.
- Keep state claims factual and one line. Never paste raw chats, model reasoning, source code, secrets, or untrusted multiline text into durable state.
- Stop for destructive actions, production changes, credentials, paid services, security uncertainty, unresolved product choices, repeated blockers, or unexplained validation failures.
