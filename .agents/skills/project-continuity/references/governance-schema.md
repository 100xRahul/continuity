# Evidence gate schema

These optional, committed records preserve source-backed engineering knowledge without storing chat history or hidden harness memory.

## Fact Cards in `PROJECT_STATE.md`

```md
- [fresh] F-001 | source: D-001 | paths: internal/auth, api/auth | verified: 2026-07-30T12:00:00Z | evidence: Focused review passed. | summary: Auth boundary preserves token invalidation.
```

Use `fresh` or `stale`. `source` identifies the compact decision, constraint, or code-map source; `paths` are repository-relative code, plan, or policy evidence paths. A fresh card turns stale only when its paths change during an evidence-aware synchronization or integration. Continuity never edits the claim itself or turns it fresh automatically.

From an unclaimed integration checkout, add a fact with `continuity fact add`, or record a fresh review with `continuity fact confirm --id F-001 --evidence 'review result'`. Claiming overlapping work is rejected while the card is stale.

## Blocker Cards in `PROJECT_STATE.md`

```md
- [open] B-001 | task: T-014 | scope: internal/auth | hypothesis: Test identity provider is unreachable. | evidence: Connection refused from focused test. | next: Obtain a reachable local provider. | resolution: none
```

Use `open` or `resolved`; a resolved card has compact new evidence in `resolution`. Add a blocker only after current progress stops. The same task, hypothesis, and evidence cannot be added again: retry only with new evidence or a new hypothesis. Open cards block overlapping claims.

## Policy checks in `CONTINUITY_POLICY.md`

```md
# Continuity policy checks

## Checks

### P-001
Scope: internal/auth
Phase: finalize
Command: go test ./internal/auth
Reason: Authentication changes require focused verification.
```

Only `finalize` is currently supported. Add a policy only with an explicit user-approved `continuity policy add --approved ...` receipt. Relevant checks run before a completion is created; a failure checkpoints the claim and requires inspection or a durable blocker rather than silently finishing work. Policy output is not copied into durable state.
