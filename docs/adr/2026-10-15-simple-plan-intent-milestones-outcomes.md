# Keep the plan to Intent, Milestones, and Outcomes

- Date: 2026-10-15
- Paths: PROJECT_PLAN.md, internal/continuity/plan.go, cmd/continuity/context.go
- Supersedes: 2026-07-30-roadmap-in-project-plan.md

## Context

- The user wants agents to know what to build next without being told, and asked to keep the plan file.
- Its Sources, Constraints, Assumptions, Decisions, Deltas, and approval receipts duplicated docs/adr and PROJECT_STATE.md or served the deleted task lifecycle.

## Decision

- PROJECT_PLAN.md holds only Intent, Milestones (planned, one active, done), and Outcomes (open or done), in priority order.
- continuity context shows the active milestone's open outcomes as Next. Agents mark outcomes done after verifying acceptance; only the user adds or changes outcomes and milestones.
- The plan is excluded from the source fingerprint, so marking an outcome done does not make the index stale.

## Reconsider when

- Agents change the plan without approval, and an enforced approval step becomes necessary.
