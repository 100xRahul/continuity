# Delivery plan schema

`PROJECT_PLAN.md` is the committed detailed delivery contract. Its front matter contains `schema`, `status`, `active_milestone`, `approved_at`, and `updated_at`.

Required sections are `Intent`, `Sources`, `Milestones`, `Outcomes`, `Constraints`, `Assumptions`, `Decisions`, and `Deltas`.

The plan remains a draft until a user explicitly approves an executable milestone with:

```text
continuity plan approve --approved --milestone M-001
```

The intake agent inspects repository facts, interviews the user one decision at a time, and stores only compact distilled facts—never raw chat. High-impact open decisions that affect the selected milestone block approval.

Milestones group user-visible outcomes. Later implementation tasks must link to an active approved outcome. Their task records carry a focused repository-relative scope contained by the outcome scope, an observable acceptance condition, and a validation command:

```text
- [ ] T-001 | focused technical step | outcome: O-001 | scope: internal/auth | depends: none | autonomous: yes | validates: go test ./internal/auth | accepts: Focused tests pass. | derived: Why this is the best next technical step.
```

`continuity ready` ranks unclaimed ready tasks using outcome priority and direct dependency unblocking evidence. A planned `continuity start` requires a one-line `--selection-reason`; its task contract supplies goal, scope, acceptance, and validation by default. From an unclaimed integration checkout, an agent may add a necessary technical task with `continuity task add --derived ...` only when it names the active outcome, remains within its scope, has validation, and records a rationale. The command commits that queue record immediately. Product, priority, constraint, and milestone changes still require plan deltas. After every linked task is integrated, `continuity plan outcome complete --id O-001 --evidence 'verification: pass'` records an evidence-backed outcome archive. `continuity plan activate --approved --milestone M-002` snapshots the completed milestone before switching active context. Completed history is preserved under `PROJECT_HISTORY/`; `PROJECT_STATE.md` retains only the compact active projection.
