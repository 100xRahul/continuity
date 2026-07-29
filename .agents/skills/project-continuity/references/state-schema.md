# State schema

`PROJECT_STATE.md` is a bounded, committed project projection. It is not a live multi-agent scratchpad.

Required front matter:

```yaml
schema: 1
fingerprint: sha256:...
branch: main
head: <observed base commit or unborn>
status: ready|working|blocked
active_task: T-001|none
updated_at: RFC3339 timestamp
```

Required sections are Product Intent, Approved Outcomes, Active Work, Ready Queue, Active Decisions, Constraints and Approval Boundaries, Code Map, Known Pitfalls and Failed Approaches, Verification, Recent Material Changes, and Handoff. Headings and front-matter keys must be unique.

## Task queue syntax

Use exactly one line per task:

```md
- [ ] T-002 | Issue previous reset tokens on password change | depends: T-001 | autonomous: yes | accepts: Focused reset-token tests pass and old tokens are rejected.
```

Status markers are `[ ]` queued and `[x]` integrated. `[>]` is a legacy single-worker marker and must match `active_task`; new concurrent work uses local claims instead.

## Live coordination

For sessions on one machine, the CLI stores claims, heartbeats, and pending completions in Git's common directory. Linked worktrees therefore see the same live board without committing it or creating merge conflicts.

A claim contains task ID, owner/session identity, worktree, branch, scope, goal, next action, and lease. A claim is exclusive by task, worktree, and overlapping scope. A completion records a verified commit until `continuity integrate` confirms that commit is in the integration checkout.

## Ownership

User-owned: Product Intent, Approved Outcomes, non-goals, priorities, and approval boundaries.

Agent-maintained: task decomposition, claims, code map, verification, factual handoffs, recent material changes, durable engineering decisions, and failures.

Do not retain raw chats, model reasoning, pasted code, secrets, generic advice, or untrusted multiline text. Add a fact only when it affects a later agent's safe behavior.

## Evidence and freshness

Use repository-relative paths in decisions, failures, code map entries, and claim scopes. A matching fingerprint is necessary but not sufficient: inspect changed evidence anchors before trusting their associated rationale. Fingerprints use Git's tracked/unignored files, so worktree metadata and ignored build artefacts do not create false staleness.
