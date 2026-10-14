# Plan schema

PROJECT_PLAN.md has front matter with schema: 2 and exactly three sections: Intent, Milestones, and Outcomes. File order is priority.

    - [ ] M-001 | title: Local storage | accepts: Data survives a restart.
    - [ ] O-001 | milestone: M-001 | title: Persist sessions | accepts: A restarted process reads earlier sessions.

A milestone is planned [ ], active [>], or done [x]; at most one is active. An outcome is open [ ] or done [x]. continuity context shows the active milestone's open outcomes as the next work.

Agents mark an outcome [x] after its acceptance is verified. Only the user adds or changes outcomes and milestones or chooses the active one. An empty plan means: interview the user one decision at a time, with a recommendation each time, and draft milestones and outcomes for approval.
