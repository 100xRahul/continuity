# Delivery plan schema

`PROJECT_PLAN.md` is the committed detailed delivery contract. Its front matter contains `schema`, `status`, `active_milestone`, `approved_at`, and `updated_at`.

Required sections are `Intent`, `Sources`, `Milestones`, `Outcomes`, `Constraints`, `Assumptions`, `Decisions`, and `Deltas`.

The plan remains a draft until a user explicitly approves an executable milestone with:

```text
continuity plan approve --approved --milestone M-001
```

The intake agent inspects repository facts, interviews the user one decision at a time, and stores only compact distilled facts—never raw chat. High-impact open decisions that affect the selected milestone block approval.

Milestones group user-visible outcomes. Later implementation tasks must link to an active approved outcome. Product, priority, constraint, and milestone changes require plan deltas. Completed milestone detail is preserved under `PROJECT_HISTORY/`; `PROJECT_STATE.md` retains only the compact active projection.
