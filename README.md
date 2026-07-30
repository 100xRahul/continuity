# Continuity

Continuity is a local, Git-backed project-memory and coordination system for coding agents. It answers two different questions without using cloud memory, embeddings, a vector database, MCP, or saved chat transcripts:

- What is this project, what has been approved, why does the code look this way, and what is next?
- Is another local agent already working on this task, worktree, or code area right now?

The detailed product roadmap belongs in committed `PROJECT_PLAN.md`; its compact active projection, source-linked facts, and durable blockers belong in `PROJECT_STATE.md`. Live ownership belongs in a tiny coordination store under Git's common directory. Optional user-approved executable checks live in committed `CONTINUITY_POLICY.md`. Separating these concerns is what makes parallel sessions safe without turning routine prompts into a bloated scratchpad.

## The rule for multiple sessions

| Situation | Correct practice |
| --- | --- |
| Two sessions in one checkout | One writer only. The second session must be read-only, wait, or use another worktree. |
| Two sessions in separate worktrees on one machine | Claim different approved tasks with non-overlapping scopes. Each worker commits independently. |
| Codex child agents in one parent session | Treat children as read-only in the parent checkout. A child writer needs a separate worktree and claim. |
| Same task in two worktrees | The second claim is rejected. |
| Overlapping files/directories | The second claim is rejected even when task IDs differ. |
| Two active writers on one branch | The second claim is rejected even when worktrees and scopes differ. |
| Separate laptops/clones | This release does not coordinate them; use a remote claim service/branch or keep one writer. |

Claims are small local records containing task ID, session/owner, worktree, scope, heartbeat, next action, and lease. Every linked Git worktree sees the same records because they live under Git's shared metadata directory rather than inside a branch.

## Optional Worktrunk bridge

[Worktrunk](https://worktrunk.dev/) is optional tooling for creating, switching, merging, and removing Git worktrees. Continuity does not depend on it: plain Git remains the portable baseline.

If you use Worktrunk, review [`.agents/worktrunk/wt.toml.example`](.agents/worktrunk/wt.toml.example) and copy it to `.config/wt.toml`. Its `pre-start` hook prints bounded Continuity orientation and live ownership in each new worktree. It does **not** claim work automatically, because the correct harness owner and narrow edit scope must be explicit. Follow the complete workflow in [`.agents/worktrunk/README.md`](.agents/worktrunk/README.md).

## Install

Build with Go 1.26 or newer and put `continuity` on your `PATH`:

```bash
go build -o continuity ./cmd/continuity
continuity init --project "Example" --objective "The approved outcome."
```

Commit `PROJECT_PLAN.md`, `PROJECT_STATE.md`, `AGENTS.md`, `.agents/skills/`, `.codex/hooks.json`, and the source. Do not commit the compiled binary or Git's local coordination store.

## Project intake

`continuity init` creates a draft `PROJECT_PLAN.md`. The agent inspects repository facts, interviews you one decision at a time where product judgment is required, and records compact outcomes, constraints, assumptions, and decision cards. It does not preserve raw chat.

After you approve the proposed first milestone, the agent records an auditable, immediate Git receipt:

```text
continuity plan approve --approved --milestone M-001
```

Only the approved active milestone becomes executable. Later roadmap items remain visible for orientation but cannot silently become agent work.

## Normal workflow

```text
new session
  -> continuity context --owner <harness:session-id>
  -> continuity status
  -> continuity ready
  -> continuity start --id T-001 --owner <harness:session-id> --selection-reason "why this is the best ready step"
  -> work + continuity checkpoint
  -> commit + continuity finalize --commit <commit> --evidence 'test: pass' # runs relevant approved policy checks
  -> merge/handoff worker commit
  -> continuity integrate --id T-001
  -> commit PROJECT_STATE.md PROJECT_HISTORY/changes/T-001.md
  -> continuity plan outcome complete --id O-001 --evidence 'outcome verification: pass' # after every linked task is integrated
```

`start` claims a task but does not mutate the committed task board. `finalize` records a verified worker result locally. `integrate` is the only operation that marks the task `[x]` in `PROJECT_STATE.md`, and it refuses until the worker commit is present in the integration checkout. It derives the actual Git paths changed during the claim, writes a permanent record under `PROJECT_HISTORY/changes/`, refreshes a short scoped change capsule, and directs the handoff to the next ready task. This prevents a background worker from falsely declaring shared work complete before its code exists on the branch everyone uses.

## Codex sessions

The repository includes optional Codex hooks:

- `SessionStart` and `SubagentStart` inject a bounded live context packet: active claims, the current worktree owner, relevant project intent, constraints, and the next safe action.
- `UserPromptSubmit` emits a very small live pulse before a later prompt, so an already-open session notices a newly claimed checkout without repeatedly receiving the full index.
- `PreToolUse` protects both Codex's `apply_patch` path and potentially mutating Bash commands. A second session may still use a small read-only inspection set, but it cannot write, revert, build into the checkout, or run an arbitrary shell command until it owns the worktree claim.

Trust the project-local hooks once with `/hooks`. Hooks are a convenience and guardrail, not the authority: the portable CLI and claim store remain the source of truth. They cover Codex's documented Bash and patch tool paths, but no hook should be treated as a complete sandbox against manual or external-process writes.

If hooks are unavailable, run `continuity context` and `continuity status` at the start of every session. A fresh session never needs to reconstruct the whole codebase before it knows whether another worker is active.

## Commands

```text
continuity brief                  print the full durable project index
continuity context                print bounded live context; use --scope for relevant capsules
continuity status                 show active claims and pending completions
continuity check                  validate durable state, task graph, and freshness
continuity reconcile              list relevant changes since durable synchronization
continuity sync                   refresh durable Git facts after reconciliation
continuity ready                  rank unclaimed autonomous ready tasks with selection evidence
continuity task add               record a user-specified or plan-bounded derived task before any claim exists
continuity plan                   create, validate, approve, activate, and inspect the delivery plan
continuity plan outcome complete  record evidence-backed completion after every linked task is integrated
continuity policy                add, inspect, or run explicitly approved finalization checks
continuity fact                  add, confirm, or inspect source-linked engineering facts
continuity blocker               record, resolve, or inspect durable no-retry blockers
continuity start                  atomically claim a task/worktree/scope
continuity heartbeat              refresh an owned claim lease
continuity checkpoint             save factual in-progress claim state
continuity release                release an owned claim or explicitly recover a stale one
continuity recover-lock           explicitly recover an abandoned internal mutex
continuity finalize               record a committed, verified worker result
continuity integrate              project a merged worker result into PROJECT_STATE.md
```

Legacy repositories can keep the compact queue format:

```md
- [ ] T-014 | Hash password reset tokens | depends: T-013 | autonomous: yes | accepts: Focused tests pass and plaintext tokens are absent from storage.
```

Only `autonomous: yes` tasks can be claimed by an agent. For legacy tasks, an omitted claim scope means `.`—the whole repository—and intentionally prevents parallel work.

After a plan is approved, executable tasks use a focused contract instead:

```md
- [ ] T-014 | Hash password reset tokens | outcome: O-003 | scope: api/auth, internal/auth | depends: T-013 | autonomous: yes | validates: go test ./api/auth ./internal/auth | accepts: Focused tests pass and plaintext tokens are absent from storage. | derived: It closes the highest-risk storage gap before endpoint work.
```

The task's outcome must belong to the active milestone, its scope must be contained by that outcome's scope, and it must name a focused validation command. `continuity ready` orders candidates by active-outcome priority, direct unblocking value, then task ID; the agent still makes the choice and records its evidence with `--selection-reason` when it claims the task. Planned `start` calls default the goal, acceptance, validation, and scope from the durable task contract, so the agent does not have to restate them.

From an unclaimed integration checkout, a user may add a specified planned task with `--approved`. An agent may instead create a needed technical subtask only inside an already-active outcome:

```text
continuity task add --derived --id T-014 --title "Hash password reset tokens" \
  --outcome O-003 --scope api/auth,internal/auth \
  --validates "go test ./api/auth ./internal/auth" \
  --accepts "Focused tests pass and plaintext tokens are absent from storage." \
  --rationale "It closes the highest-risk storage gap before endpoint work."
```

That command immediately commits the queue record, so other worktrees and harnesses can see it. It cannot add product scope: it rejects a draft plan, inactive or unknown outcomes, broad scopes, missing validation, and a missing derivation rationale. In a legacy repository without `PROJECT_PLAN.md`, `continuity task add --approved ...` remains the explicit user-approval route. A writer claim also requires a named Git branch, and a second active writer on that branch is rejected.

## State and freshness

`PROJECT_STATE.md` has a default 120,000-byte budget: approximately 30,000 tokens using a deliberately conservative four-bytes-per-token estimate. This is a durable-storage budget, not an instruction to inject the entire file into every session. It holds current, load-bearing facts rather than a diary. `PROJECT_PLAN.md` holds the detailed roadmap and approval contract; completed milestone detail can move to committed `PROJECT_HISTORY/` archives.

### Layered context, not 30k-token startup prompts

Use `Context Capsules` inside the state for richer, path-scoped durable knowledge:

```md
### Authentication boundary
Paths: api/auth, internal/auth
Summary: Token rotation and session invalidation contract.
Details: Load this only for authentication work; keep the precise invariants and links here.
```

`continuity context --scope internal/auth --limit 12000` selects the matching capsule plus project-wide orientation. When a worker has a live claim, its claim scope is selected automatically. An ordinary startup packet remains small; use a larger `--limit` only when the task needs deeper durable context. `continuity brief` intentionally prints the whole index and should be used for deliberate review, not routine startup.

Integration keeps only the three newest generated `Change T-*` capsules in the active index. Each one carries intent, changed paths, validation evidence, next work, and a link to its full committed archive. A later agent working in a matching path sees the short record without rereading the previous code; older records remain available under `PROJECT_HISTORY/changes/` on demand. Once all outcome-linked tasks are integrated, `continuity plan outcome complete` writes `PROJECT_HISTORY/outcomes/O-001.md`; activating the next milestone similarly preserves `PROJECT_HISTORY/milestones/M-001.md` before changing active context.

The source fingerprint uses Git's tracked and unignored files. It does not hash `.git` worktree metadata, ignored build binaries, generated harness adapters, or the target of an external symlink. A matching fingerprint avoids broad rescans; evidence anchors still tell an agent which small areas need review.

The CLI rejects duplicate front matter or Markdown sections and rejects multiline lifecycle fields, so error text or prompt injection cannot silently create a second `Approved Outcomes` section.

## Evidence gates: facts, policies, and blockers

These three small, committed structures keep the index current without turning it into a chat log.

- A **Fact Card** is a short statement that later work relies on, tied to its source paths. Add one only when the fact affects a later agent's safe behavior. When those paths change during `continuity sync`, a plan or policy update, or worker integration, Continuity marks the fact `stale`; it never silently changes its meaning or claims it was reviewed. A task whose scope overlaps a stale fact cannot be claimed until an unclaimed integration checkout records `continuity fact confirm --id F-001 --evidence 'review result'`.
- A **policy check** is a versioned shell command in `CONTINUITY_POLICY.md`. It exists only after an explicit `continuity policy add --approved ...` receipt, so an agent cannot invent executable local policy. Relevant checks run at `finalize`; a failure checkpoints the claim and refuses to create a completion. Use `continuity policy check --scope path` to run the same selected checks deliberately.
- A **Blocker Card** records a meaningful stall: the task/scope, current falsifiable hypothesis, actual evidence, and the next action that can create new evidence. It is not a generic todo list. An open blocker prevents overlapping tasks from being claimed. The same task, hypothesis, and evidence cannot be recorded again: the next attempt needs new evidence or a new hypothesis. Resolve it with `continuity blocker resolve --id B-001 --evidence 'new evidence'`.

The compact on-disk syntax and ownership rules are in [the governance schema](.agents/skills/project-continuity/references/governance-schema.md). This is deliberately conservative: automatic behavior can only make a fact require review; it cannot fabricate approval, completion, or a fresh review result.

## Harness support

The portable layer is `PROJECT_STATE.md`, `AGENTS.md`, `.agents/skills/project-continuity`, and the CLI. Repository adapters point Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode to that same protocol. Use `--owner harness:session-id` with the CLI outside Codex; `--session` is only a Codex convenience alias.

The live claim/heartbeat feature currently targets same-machine local Git worktrees. Codex hooks are the first native accelerator because they can automatically inject coordination context and guard Codex write paths; the core CLI itself stays harness-neutral.

Codex documents that child-agent hooks use the parent session ID. That makes a session ID insufficient to safely distinguish two simultaneous child writers in one checkout, so Continuity deliberately treats same-checkout child agents as research/review workers. Use a worktree for every independent writer.

## Safety model

- Users own product intent, approved outcomes, priorities, and approval boundaries.
- Agents maintain factual claims, code map, verification, handoffs, decisions, and durable failures.
- Agents do not invent tasks, features, approval, or completion evidence.
- Agents stop for destructive or production actions, credentials, paid services, security uncertainty, unresolved product choices, unexplained validation failures, or repeated blockers.
- Code remains the implementation authority. Durable state points to relevant code; it does not replace targeted inspection or verification.
