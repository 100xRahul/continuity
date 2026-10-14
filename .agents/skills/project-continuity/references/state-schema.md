# State schema

PROJECT_STATE.md starts with front matter: schema (4), fingerprint, branch, head, and updated_at. continuity sync maintains them.

Required sections: Constraints and Approval Boundaries, Code Map, Fact Cards, Context Capsules, Known Pitfalls and Failed Approaches, and Handoff. Product intent and outcomes live in PROJECT_PLAN.md; decisions live in docs/adr.

Fact Cards use one line each:

    - [fresh] F-001 | source: docs/adr/2026-01-31-title.md | paths: internal/auth, api/auth | verified: 2026-07-30T12:00:00Z | evidence: Focused review passed. | summary: Auth boundary preserves token invalidation.

A fresh fact becomes stale when continuity sync sees changes under its paths. Only continuity fact confirm with new review evidence makes it fresh again, and the restart context lists stale facts.

Context Capsules hold path-scoped detail:

    ### Authentication boundary
    Paths: api/auth, internal/auth
    Summary: Token rotation and session invalidation contract.
    Details: Load only for authentication work.

Known Pitfalls are bullets; put evidence paths in backticks so continuity recall --path and the hooks link them to files. The Handoff is free text: what was done, what is in progress, and the exact next step.

The file may hold about 30,000 tokens; the restart context selects parts of it. The source fingerprint ignores PROJECT_PLAN.md, docs/adr, and harness adapters, so updating them does not make the index stale.
