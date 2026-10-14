---
schema: 2
---

# Continuity plan

## Intent

Give every coding-agent session, in any harness, the project's decisions, plan, and recent work without the user explaining them again, using local files that live with the code.

## Milestones

- [x] M-001 | title: Portable continuity core | accepts: A dependency-free Go CLI keeps a compact Git-backed index that every supported harness reads.
- [>] M-002 | title: Decision memory without coordination overhead | accepts: A fresh Claude Code or Codex session sees current decisions, the next work, and the last handoff without the user explaining them; claims and the task lifecycle are gone.

## Outcomes

- [x] O-001 | milestone: M-001 | title: CLI that keeps and validates a compact Git-backed index | accepts: init, check, reconcile, and sync work on a fresh repository.
- [x] O-002 | milestone: M-001 | title: One protocol with adapters for Codex, Claude Code, Cursor, Copilot CLI, Gemini CLI, and OpenCode | accepts: init points every harness at the same instructions without overwriting existing files.
- [x] O-003 | milestone: M-001 | title: Selective context within a 30k-token durable budget | accepts: The restart packet stays bounded while capsules hold detail.
- [x] O-004 | milestone: M-002 | title: Decisions recorded as ADR files and shown to every new session | accepts: The restart packet lists current decision titles, hooks show path-linked decisions once per session, and recall finds decisions, pitfalls, history, and commits.
- [x] O-005 | milestone: M-002 | title: Remove claims, the task lifecycle, blockers, policies, and the Worktrunk bridge | accepts: The CLI keeps only memory commands and all tests pass.
- [x] O-006 | milestone: M-002 | title: Reduce the plan to Intent, Milestones, and Outcomes | accepts: continuity context shows the active milestone's open outcomes as Next.
- [ ] O-007 | milestone: M-002 | title: Verify fresh sessions in Claude Code and Codex on the Mac and the Orca VPS | accepts: The user sees decisions and Next in a new session without explaining them again.
