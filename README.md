# Continuity

Continuity is a local, Git-backed project-state system for coding agents. It gives a new session or different harness a compact, current answer to: what this project is, what is active, why key decisions exist, what failed, what must be verified, and what authorized work comes next.

It deliberately does **not** use a cloud service, vector database, embeddings, MCP, or captured chat transcripts. `PROJECT_STATE.md` is the shared project brain; Git is the history and recovery mechanism.

## Core workflow

```text
continuity init --project "My project" --objective "The approved outcome."
continuity brief
continuity start
agent works and verifies
continuity checkpoint or continuity finalize
```

`continuity init` creates `PROJECT_STATE.md` and missing adapters without overwriting any existing instruction, skill, or harness configuration. `continuity brief` is the entry point for every new session. It emits the bounded state and warns when the state is stale. `continuity reconcile` identifies only the committed and uncommitted changes since the last state synchronization, avoiding a broad repository rescan.

## Install

Build the CLI with Go 1.26 or newer:

```bash
go build -o continuity ./cmd/continuity
./continuity init --project "Example" --objective "Build the approved outcome."
```

Commit `PROJECT_STATE.md`, `AGENTS.md`, `.agents/skills/`, and the source. Do not commit the compiled `continuity` binary.

`init` preserves an existing `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, Copilot instruction file, Cursor rule, OpenCode config, or skill directory. Add the short continuity protocol to any preserved user-owned instruction file instead of allowing a tool to overwrite it.

## Commands

```text
continuity brief                 print the bounded state and freshness status
continuity check                 validate schema, task graph, fingerprint, and anchors
continuity reconcile             list only changes requiring state reconciliation
continuity sync                  refresh state branch, HEAD, fingerprint, and timestamp after commits
continuity ready                 list unblocked autonomous tasks
continuity start                 declare one task active with its execution contract
continuity checkpoint            preserve factual in-progress handoff state
continuity finalize              close the active task and expose the next one
```

Every task line has one fixed form:

```md
- [ ] T-014 | Hash password reset tokens | depends: T-013 | autonomous: yes | accepts: Focused tests pass and plaintext tokens are absent from storage.
```

`accepts:` is required for autonomous work. A task may be ready only when each dependency is `[x]` and no other task is active.

## State contract

`PROJECT_STATE.md` has a fixed byte budget (24 KiB by default) and records current truth rather than an append-only diary. It contains product intent, approved outcomes, active work, a dependency-aware task queue, decisions, constraints, code map, durable failures, verification, recent material changes, and a handoff.

The source fingerprint excludes state and harness adapters. It changes when source, tests, or project configuration changes; therefore an agent can trust a matching state but must reconcile a mismatch before proceeding.

## Harness support

The portable layer is `PROJECT_STATE.md`, `AGENTS.md`, `.agents/skills/project-continuity`, and the CLI. It works even when no agent has hook support.

Repository adapters are included for Codex, Claude Code, Cursor, GitHub Copilot CLI, Gemini CLI, and OpenCode. They contain no duplicate project memory: each directs the harness back to the same state and workflow.

| Harness | Adapter |
| --- | --- |
| Codex | Root `AGENTS.md` and `.agents/skills/project-continuity/` |
| Claude Code | `CLAUDE.md` plus `.claude/skills` symlinked to the shared skill |
| Cursor | Root `AGENTS.md` and `.cursor/rules/project-continuity.mdc` |
| GitHub Copilot CLI | `AGENTS.md`, `.github/copilot-instructions.md`, and shared skill |
| Gemini CLI | `GEMINI.md` pointing to the same protocol |
| OpenCode | `AGENTS.md` and `opencode.json` loading it explicitly |

Native hooks are intentionally optional accelerators because their configuration and trust models differ by harness. Correct continuation must work with only the portable layer.

## Safety model

- Users own product intent, approved outcomes, and approval boundaries.
- Agents maintain active work, code map, evidence, handoffs, and engineering-level decisions.
- Agents may execute only explicit, unblocked tasks marked `autonomous: yes`.
- Agents stop for product ambiguity, destructive or production work, credentials, paid services, security uncertainty, unexplained validation failures, or repeated blockers.
- Code remains the implementation authority. The state tells an agent where and why to inspect; it never replaces narrow verification of relevant code.
