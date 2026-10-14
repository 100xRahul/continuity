# Record decisions as ADR files and recall them by path and topic

- Date: 2026-10-15
- Paths: docs/adr, internal/continuity/decisions.go, internal/continuity/search.go, internal/continuity/state.go, cmd/continuity/recall.go, .agents/hooks
- Supersedes: 2026-07-30-durable-state-and-live-ownership.md
- Related: task T-017

## Context

- New sessions acted as if no past decision existed. The restart packet was 1,157 bytes and carried none of the 15 recorded decisions; an agent saw them only if it chose to read the whole state file.
- A decision made in conversation had no capture path, and decisions explained only in commit messages never reached the index.

## Constraints and evidence

- Local and offline; works across harnesses; no vector database or embeddings (project rule since 2026-07-30).
- Keyword (BM25) search over this repository's records found the right record in the top 3 for 10 of 12 test questions, but per-prompt automatic injection scored an unrelated prompt above three real matches.
- Research: similarity search returns 6–27% of the right answers on "which decision is current" questions, against 98–100% with explicit supersession links (MOOSEDev, arXiv 2608.13662); outdated records stay retrievable and must be labelled (PACMI, 2610.05732); file-anchored memory raised a coding benchmark by 21 points (MemTrace, 2610.04838); always-loaded context degrades as it grows (2610.11007); decisions extracted from transcripts after the fact are unreliable (GADR, 2608.17694).

## Decision

- Each decision is one Markdown file in `docs/adr/`, named by date, with `Date`, `Paths`, and optional `Supersedes` fields. A record is current until a later record supersedes it; no status field is stored.
- `PROJECT_STATE.md` stays the compact project index.
- The restart packet lists the titles of current decisions. Claude Code and Codex hooks show the decisions and pitfalls linked to a file when the agent reads or edits it, once per session. `continuity recall` searches decisions, pitfalls, facts, history, and commit messages.
- The agent in the conversation records decisions when the user states or approves them; no extra model reads transcripts.

## Alternatives considered

- claude-mem: per-machine database outside Git, LLM compression through a hosted service by default, records events rather than approved decisions.
- claude-sessions: Claude Code only; session summaries rather than decisions.
- One-line decisions inside `PROJECT_STATE.md`: parallel branches conflict on the same lines, and the file grows with history.
- SQLite full-text search: the same BM25 ranking, plus a binary file that Git cannot diff and an extra dependency.
- Automatic recall on every prompt: measured noise above real matches.

## Consequences

- Decisions survive harness switches, are reviewed in diffs, and old records remain readable as history.
- Capture depends on the agent following the protocol.

## Reconsider when

- Decisions recorded in conversation are found missing from `docs/adr/` in practice.
- The number of current decisions makes the restart index too large to stay always loaded.
