# State schema

`PROJECT_STATE.md` contains YAML-like front matter followed by fixed Markdown sections.

Required front matter:

```yaml
schema: 1
fingerprint: sha256:...
branch: main
head: <git commit or unborn>
status: ready|working|blocked
active_task: T-001|none
updated_at: RFC3339 timestamp
```

Required sections are Product Intent, Approved Outcomes, Active Work, Ready Queue, Active Decisions, Constraints and Approval Boundaries, Code Map, Known Pitfalls and Failed Approaches, Verification, Recent Material Changes, and Handoff.

## Task queue syntax

Use exactly one line per task:

```md
- [ ] T-002 | Issue previous reset tokens on password change | depends: T-001 | autonomous: yes | accepts: Focused reset-token tests pass and old tokens are rejected.
```

Status markers are `[ ]` queued, `[>]` active, and `[x]` complete. Dependencies are task IDs or `none`. Only tasks marked `autonomous: yes` with explicit observable `accepts:` criteria can be selected by a fresh agent without a new instruction.

## Ownership

User-owned: Product Intent, Approved Outcomes, non-goals, priorities, and approval boundaries.

Agent-maintained: Active Work, Ready Queue decomposition, Code Map, Verification, recent material changes, durable engineering decisions, failures, and Handoff.

Do not retain raw chats, model reasoning, pasted code, secrets, generic advice, or a complete task diary. Add a fact only when it affects a later agent's safe behavior.

## Evidence and freshness

Use repository-relative paths in decisions, failures, code map entries, and execution scopes. If an anchored area changes, review the related fact instead of blindly trusting it. The CLI fingerprint excludes state and adapter files, so state-only edits do not cause false staleness.
