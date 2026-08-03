# State schema

`PROJECT_STATE.md` is a bounded, committed operational projection. `PROJECT_PLAN.md` is the detailed delivery contract; this file keeps only its current executable projection. Neither document is a live multi-agent scratchpad.

Required front matter:

```yaml
schema: 2
fingerprint: sha256:...
branch: main
head: <observed base commit or unborn>
status: ready|working|blocked
active_task: T-001|none
updated_at: RFC3339 timestamp
```

Required sections are Product Intent, Approved Outcomes, Active Work, Ready Queue, Active Decisions, Constraints and Approval Boundaries, Code Map, Context Capsules, Known Pitfalls and Failed Approaches, Verification, Recent Material Changes, and Handoff. Headings and front-matter keys must be unique.

The default durable budget is 120,000 bytes, approximately 30,000 tokens under a four-bytes-per-token estimate. It is a storage ceiling, not routine prompt size.

## Context capsules

Use level-three headings inside `Context Capsules`:

```md
### Module name
Paths: path/to/module, other/relevant/path
Summary: One-line fact needed to decide relevance.
Details: Compact durable details, invariants, decisions, and references for this scope.
```

Paths must be repository-relative. `Paths: .` is project-wide orientation. `continuity context --scope path/to/module` selects matching capsules; an active claim's scope is selected automatically. Do not use capsules for raw chats, reasoning traces, or pasted source code.

The context packet is priority-structured rather than byte-truncated. Ownership, state/fact/blocker gates, constraints, and the exact next action must fit as one safety block or `continuity context` returns an explicit limit error. Plan projection and capsule detail are optional blocks and may be condensed or omitted whole. A gated ready task is reported with its recovery action and is never printed as claimable.

## Task queue syntax

Legacy repositories use one compact line per task:

```md
- [ ] T-002 | Issue previous reset tokens on password change | depends: T-001 | autonomous: yes | accepts: Focused reset-token tests pass and old tokens are rejected.
```

Status markers are `[ ]` queued and `[x]` integrated. `[>]` is a legacy single-worker marker and must match `active_task`; new concurrent work uses local claims instead.

After a delivery plan is approved, executable tasks use an outcome-bound contract:

```md
- [ ] T-002 | Issue previous reset tokens on password change | outcome: O-001 | scope: api/auth, internal/auth | depends: T-001 | autonomous: yes | validates: go test ./api/auth ./internal/auth | accepts: Focused reset-token tests pass and old tokens are rejected. | derived: It unblocks the highest-priority security outcome.
```

The outcome must be active, each task path must be contained by the outcome's declared scope, and `validates` must name focused verification. `continuity ready` ranks candidates by outcome priority and direct unblocking value. A planned `continuity start` defaults its goal, acceptance, scope, and validation from this record and requires a compact `--selection-reason`.

## Live coordination

For sessions on one machine, the CLI stores claims, heartbeats, and pending completions in Git's common directory. Linked worktrees therefore see the same live board without committing it or creating merge conflicts.

A claim contains task ID, owner/session identity, worktree, branch, base commit, scope, goal, next action, selection reason, and lease. A claim is exclusive by task, worktree, and overlapping scope. A completion retains that task contract plus a verified commit until `continuity integrate` confirms that commit is in the integration checkout.

Integration derives changed paths from the recorded base commit through the verified result commit. It writes a permanent `PROJECT_HISTORY/changes/T-001.md` record and refreshes a generated `Change T-001` capsule with intent, changed paths, validation evidence, next work, and the archive path. Preserve only the newest three generated change capsules in the active index; archives remain committed and on-demand.

Integration also compacts archive-backed repetition in the operational index: preserve every unfinished task, completed tasks linked to incomplete outcomes, unarchived legacy contracts, and the newest 12 archive-backed completed task contracts, plus the newest 12 distinct lines in `Verification` and `Recent Material Changes`. When an older completed task leaves the active queue, remove its already-satisfied dependency edge from retained tasks. Git and `PROJECT_HISTORY/` preserve full history, and an archived task ID cannot be reused. Never infer that an engineering decision, failure, Fact Card, or Blocker Card is obsolete; those require their explicit lifecycle.

## Ownership

User-owned: Product Intent, Approved Outcomes, non-goals, priorities, and approval boundaries.

Agent-maintained: task decomposition, claims, code map, verification, factual handoffs, recent material changes, durable engineering decisions, and failures.

Do not retain raw chats, model reasoning, pasted code, secrets, generic advice, or untrusted multiline text. Add a fact only when it affects a later agent's safe behavior.

## Evidence and freshness

Use repository-relative paths in decisions, failures, code map entries, and claim scopes. A matching fingerprint is necessary but not sufficient: inspect changed evidence anchors before trusting their associated rationale. Fingerprints use Git's tracked/unignored files, so worktree metadata and ignored build artefacts do not create false staleness.

## Optional evidence gates

`Fact Cards` and `Blocker Cards` are optional state sections, so existing repositories remain readable without a migration. They use strict single-line records and are validated whenever present. Fact Cards name the source and repository paths behind a compact engineering fact; an affected fresh card becomes stale during an evidence-aware synchronization or integration. A stale fact blocks only overlapping task scopes until `continuity fact confirm` records new review evidence.

Blocker Cards capture the bounded handoff for a real stall rather than an open-ended task list. An open card blocks overlapping work until new evidence resolves it. The full record syntax, policy document syntax, and command boundaries are in [governance-schema.md](governance-schema.md).
