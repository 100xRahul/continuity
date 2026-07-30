# Continuity protocol

This repository has three complementary memories:

- `PROJECT_PLAN.md` is the detailed, committed delivery contract: roadmap milestones, user-visible outcomes, inferred source authority, constraints, assumptions, decision cards, and approved plan deltas.
- `PROJECT_STATE.md` is the compact, committed operational index: current intent, active approved outcome, code map, durable failures, and integrated work.
- Git's local common directory holds short-lived shared coordination: live claims, heartbeats, and completed results awaiting integration. It is shared by local Git worktrees but is never committed or used as a chat transcript.

Treat harness memory and chat history as disposable conveniences, not authority.

## Project intake and approval

1. Inspect repository facts before asking questions. When intent, sources, constraints, priorities, or ownership are unclear, interview the user one decision at a time and give a recommendation.
2. Record compact facts, source references, assumptions, and decision cards in the draft `PROJECT_PLAN.md`; never paste raw chats or reasoning.
3. Do not claim implementation work while the plan is a draft. High-impact source conflicts or open decisions affecting the executable milestone require a user answer.
4. After the user explicitly approves, run `continuity plan approve --approved --milestone M-001`. This records and commits the approval before autonomous work begins.
5. Later product, priority, constraint, or milestone changes become an explicit plan delta. Technical implementation decomposition remains inside already-approved outcomes.

## Start or resume a session

1. Use the `project-continuity` skill when available. In Codex, trust this repository's optional hooks once with `/hooks`; a trusted SessionStart hook injects a bounded live coordination snapshot automatically.
2. Otherwise run `continuity context --owner <harness:session-id>` and `continuity status`. Codex may use `--session` as a convenience alias. Use the active claim's scope or `--scope path/to/area` to select only relevant Context Capsules; do not read the full index during routine startup.
3. If another owner has a claim in this worktree, do not edit, revert, reinterpret, or “clean up” its changes. Work read-only, wait, or open a separate Git worktree.
4. If state is stale, run `continuity reconcile`, inspect only the listed/mapped evidence, update durable facts, and run `continuity sync` before claiming new work.

## Claim before writing

- One worktree has exactly one writer. A second writer in the same checkout is unsafe even when it thinks its task is unrelated.
- Codex child agents in one parent session are read-only in that parent checkout. Their hook identity is the parent session, so a child that needs to write must receive a separate Git worktree and an independent claim.
- Before any code edit, claim one explicit autonomous task. In an approved plan, use a task linked to the active outcome and run `continuity start --id T-001 --owner <harness:session-id> --selection-reason "evidence-based reason"`; its focused scope and validation default from the task contract. Codex may use `--session` as a convenience alias.
- A claim is exclusive by task ID, worktree, and overlapping path scope. An omitted scope means the whole repository and intentionally prevents parallel work.
- Different writers require different worktrees, different branches, different approved tasks, and non-overlapping scopes. The CLI rejects a second active writer claim on the same named branch. They must not directly edit the committed project index while working.
- Use `continuity heartbeat` or a checkpoint after material progress. Release only your own claim; recover a stale claim explicitly after inspecting `continuity status`.

When no task is queued, work only from an unclaimed integration checkout. In a legacy repository, use `continuity task add --approved ...` for a user-approved task. In an approved plan, an agent may use `continuity task add --derived --outcome O-001 --scope path --validates command --rationale text ...` only for a focused technical step inside the active outcome; it cannot add product scope and immediately commits the queue record. `continuity ready` ranks ready planned tasks by outcome priority and direct unblocking value, but the claimant must record why it selected one. Worktrunk users may opt into `.agents/worktrunk/wt.toml.example` for new-worktree orientation, but claims remain explicit.

## Complete safely

1. Run the task's focused validation and commit the worker result.
2. Run `continuity finalize --commit <commit> --summary ... --evidence 'command: result'`. This creates a local completion record; it does not falsely mark the shared project index complete.
3. Merge or hand off the commit into the integration checkout, then run `continuity integrate --id T-001`. Only integration updates `PROJECT_STATE.md` and unblocks dependent work.

## Boundaries

- Do not change Product Intent, Approved Outcomes, priorities, or approval boundaries without explicit user direction.
- Do not claim `autonomous: no` work. Do not invent product features; a derived task must stay inside an active approved outcome's scope and acceptance boundary.
- Keep state claims factual and one line. Never paste raw chats, model reasoning, source code, secrets, or untrusted multiline text into durable state.
- Keep `Context Capsules` path-scoped. The durable index may hold approximately 30k tokens, but a routine context packet must stay bounded and select only project-wide plus task-relevant capsules.
- Preserve completed milestones in committed `PROJECT_HISTORY/` archives when active plan context would otherwise grow large.
- Stop for destructive actions, production changes, credentials, paid services, security uncertainty, unresolved product choices, repeated blockers, or unexplained validation failures.
