---
schema: 4
fingerprint: sha256:0c3442dd785ddc152e65c254b06576a0f07aa93b5836121959e56f26b904179c
branch: t-017-decision-recall
head: f41b836cda389f5610c0e577af2ee19311f9f350
updated_at: 2026-10-14T19:09:16Z
---

# Continuity

## Constraints and Approval Boundaries

- Do not invent product outcomes; work only on outcomes in PROJECT_PLAN.md, and the user approves every change to the plan.
- Request approval for destructive actions, production changes, credentials, paid services, or unresolved product choices.
- Do not require one vendor's hooks, memory, plugin, MCP support, or cloud service for correct continuation.
- Keep the state current and bounded; do not store raw chats, reasoning traces, or pasted source code.

## Code Map

- Commands and index maintenance — `cmd/continuity/main.go` — init, check, reconcile, sync, brief, and shared helpers.
- Restart packet — `cmd/continuity/context.go` — critical block, plan Next, handoff, current decision titles, capsules, and byte-bounded assembly.
- Recall — `cmd/continuity/recall.go`, `internal/continuity/search.go` — BM25 keyword search over durable memory and path-linked recall.
- Harness hooks — `cmd/continuity/hook.go`, `internal/continuity/shown.go`, `.agents/hooks/continuity.sh` — session-start packet and once-per-session path-linked records for Claude Code and Codex.
- Facts — `cmd/continuity/facts.go`, `internal/continuity/facts.go` — source-linked facts that age when their paths change.
- Decision records — `internal/continuity/decisions.go`, `docs/adr` — ADR parsing, supersession, and validation.
- Plan — `internal/continuity/plan.go` — Intent, Milestones, Outcomes, and the active milestone's open outcomes.
- State engine — `internal/continuity/state.go` — schema 4 sections, capsules, validation, and atomic writes.
- Git facts — `internal/continuity/git.go` — fingerprint, changes since the last sync, and commit messages.
- Adapter installer — `internal/continuity/adapters.go` — non-overwriting scaffolding, marked bridges, and hook-config merges for Claude Code and Codex.
- Tests — `cmd/continuity/*_test.go`, `internal/continuity/*_test.go`.

## Fact Cards
- [stale] F-001 | source: docs/adr/2026-10-15-keep-facts-drop-blockers-and-policies.md | paths: cmd/continuity/facts.go, internal/continuity/facts.go | verified: 2026-10-14T19:08:58Z | evidence: TestSyncAgesFactUntilAReviewConfirmsIt and TestFactCardsAgeOnlyOverlappingFreshEvidence passed. | summary: Facts age when their source paths change and become fresh again only through continuity fact confirm.
## Context Capsules

### Project-wide orientation
Paths: .
Summary: Continuity is local project memory: PROJECT_PLAN.md says what to build next, docs/adr holds decisions, and PROJECT_STATE.md indexes code, facts, pitfalls, and the last handoff.
Details: Hooks and continuity context deliver selected parts at session start; continuity recall searches the rest. Parallel agents run in Orca or separate worktrees; Continuity does not coordinate writers.

### Memory delivery
Paths: cmd/continuity/context.go, cmd/continuity/hook.go, cmd/continuity/recall.go, internal/continuity/search.go, internal/continuity/shown.go
Summary: Decision titles are always loaded; path-linked decisions, pitfalls, and facts appear once per session when a file is used; keyword recall covers everything else.
Details: Measured before this design, a new session saw 0 of 15 decisions. BM25 with the Harman S-stemmer found the right record in the top 3 for 11 of 12 test questions. Automatic search on every prompt was rejected because an unrelated prompt outscored real matches. See docs/adr/2026-10-15-decisions-as-adr-files-with-recall.md.

## Known Pitfalls and Failed Approaches

- Record only failures that remain relevant. Include the failed approach, evidence, resolution, and affected paths.
- Do not use an unanchored `.gitignore` entry named `continuity`; Git will ignore package directories with that name. Use `/continuity` for the root binary. Evidence: `.gitignore`.
- Exclude every generated harness adapter, including `opencode.json`, from the source fingerprint; otherwise a new initialized repository is stale immediately. Evidence: `internal/continuity/git.go`, `cmd/continuity/init_test.go`.
- Do not use a fixed `PROJECT_STATE.md.tmp` name for atomic writes. Concurrent processes can rename or remove each other's temporary file; use a unique same-directory temporary file and rename it atomically. Evidence: `internal/continuity/state.go`, `internal/continuity/coordination_test.go`.
- Do not walk linked-worktree `.git` metadata when fingerprinting. Its pointer file contains worktree-specific paths and creates false staleness; derive files through `git ls-files` instead. Evidence: `internal/continuity/git.go`, `internal/continuity/coordination_test.go`.
- Do not equate the approximately 30k-token durable budget with routine context size. The full index is deliberate-review material; startup and task context must select only project-wide and scope-relevant capsules. Evidence: `cmd/continuity/coordination.go`, `AGENTS.md`.
- Do not equate preserving an existing harness instruction file with integrating Continuity. Preserve its content byte-for-byte and append one idempotent marked bridge when it does not already reference the shared protocol. Evidence: `internal/continuity/adapters.go`.
- Do not write unchecked multiline text into state records. It can forge a second heading or record; fact fields accept one line each. Evidence: `cmd/continuity/main.go`, `internal/continuity/facts.go`.
- Do not silently refresh a fact after its source changes. Sync marks affected facts stale, and only `continuity fact confirm` with new review evidence makes one fresh. Evidence: `cmd/continuity/facts.go`, `internal/continuity/facts.go`.
- Do not let an ignored fingerprint path bypass fact aging. `check` and fact writes inspect changed source-linked paths separately, including instruction files such as `README.md`. Evidence: `cmd/continuity/main.go`, `cmd/continuity/facts.go`.
- Do not byte-truncate the restart packet. Keep its critical block whole and condense or omit later sections structurally. Evidence: `cmd/continuity/context.go`.
- Do not treat a backticked `.` in a pitfall as an evidence path; it linked the pitfall to every file. Evidence: `internal/continuity/state.go`.
- Do not rely on Claude Code reading AGENTS.md: when a CLAUDE.md exists, Claude Code reads only CLAUDE.md, so CLAUDE.md must import `@AGENTS.md`. Evidence: `CLAUDE.md`, `internal/continuity/adapters.go`.

## Handoff

Last session (2026-10-15): decisions became ADR files in docs/adr, shown at session start and on file use, with continuity recall for search; claims, the task lifecycle, blockers, policies, and the Worktrunk bridge were removed, and the plan was reduced to Intent, Milestones, and Outcomes.
Next step: outcome O-007 — install the rebuilt continuity binary on every machine that runs agents (this Mac and the Orca VPS), approve the project hooks in Claude Code with /hooks and trust them in Codex, then open a fresh session and confirm it shows the decisions and Next without re-explaining.
