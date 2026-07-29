package continuity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type InstallReport struct {
	Created []string
	Skipped []string
}

// InstallAdapters creates only missing repository adapters. Existing instruction
// files are never merged or overwritten because they may contain user-owned
// policy that this tool cannot safely infer.
func InstallAdapters(root string) (InstallReport, error) {
	report := InstallReport{}
	files := map[string]string{
		"AGENTS.md":                                  agentsTemplate,
		"CLAUDE.md":                                  claudeTemplate,
		"GEMINI.md":                                  geminiTemplate,
		".github/copilot-instructions.md":            copilotTemplate,
		".cursor/rules/project-continuity.mdc":       cursorTemplate,
		".codex/hooks.json":                          codexHooksTemplate,
		".codex/hooks/continuity.sh":                 codexHookScriptTemplate,
		"opencode.json":                              openCodeTemplate,
		".agents/skills/project-continuity/SKILL.md": skillTemplate,
		".agents/skills/project-continuity/agents/openai.yaml":         skillMetadataTemplate,
		".agents/skills/project-continuity/references/state-schema.md": skillSchemaTemplate,
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, relative := range paths {
		created, err := writeIfMissing(filepath.Join(root, relative), files[relative])
		if err != nil {
			return report, err
		}
		if created {
			report.Created = append(report.Created, relative)
		} else {
			report.Skipped = append(report.Skipped, relative)
		}
	}
	if err := installClaudeSkillLink(root, &report); err != nil {
		return report, err
	}
	return report, nil
}

func writeIfMissing(path, contents string) (bool, error) {
	if _, err := os.Lstat(path); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	if _, err := file.WriteString(contents); err != nil {
		return false, err
	}
	return true, nil
}

func installClaudeSkillLink(root string, report *InstallReport) error {
	path := filepath.Join(root, ".claude", "skills")
	if _, err := os.Lstat(path); err == nil {
		report.Skipped = append(report.Skipped, ".claude/skills")
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.Symlink("../.agents/skills", path); err != nil {
		report.Skipped = append(report.Skipped, ".claude/skills (symlink unavailable; use .agents/skills manually)")
		return nil
	}
	report.Created = append(report.Created, ".claude/skills -> ../.agents/skills")
	return nil
}

func (r InstallReport) String() string {
	return fmt.Sprintf("created %d adapter(s), preserved %d existing file(s)", len(r.Created), len(r.Skipped))
}

const agentsTemplate = `# Continuity protocol

This repository uses a bounded committed project index plus a local live coordination board. Treat chat history and harness-specific memory as disposable conveniences, not authority.

Before non-trivial work, use the project-continuity skill when available. Run continuity context --owner <harness:session-id> and continuity status; Codex may use --session as a convenience alias. Use an active claim's scope or --scope path/to/area to load only relevant Context Capsules. If another owner holds this worktree, do not edit, revert, or reinterpret its changes; work read-only, wait, or use a separate Git worktree. Codex child agents are read-only in their parent checkout; give a child writer a separate worktree and claim.

Before editing, claim one explicit autonomous task with continuity start and a narrow --scope. One worktree has one writer; claims also reject duplicate tasks and overlapping scopes. Checkpoint material progress or blockers. Commit and finalize with verification evidence, then merge/hand off and use continuity integrate to update PROJECT_STATE.md.

Do not change product intent, approved outcomes, or approval boundaries without user direction. Do not invent product features. Stop for destructive actions, production changes, credentials, external spending, security uncertainty, unresolved product choices, repeated blockers, or unexplained failures.
`

const claudeTemplate = `# Continuity adapter

Read and follow AGENTS.md before non-trivial work. PROJECT_STATE.md is the project authority; use the project-continuity skill or continuity CLI fallback before resuming, editing, handing off, or completing work.
`

const geminiTemplate = `# Continuity adapter

Read and follow AGENTS.md before non-trivial work. Use PROJECT_STATE.md and the continuity CLI as durable cross-harness state; do not rely on chat memory as project authority.
`

const copilotTemplate = `# Continuity adapter

@AGENTS.md

Use PROJECT_STATE.md as the durable project authority. Invoke the repository project-continuity skill or run its CLI fallback for every non-trivial task.
`

const cursorTemplate = `---
description: Enforce repository-native continuity for non-trivial work.
alwaysApply: true
---

Read and follow AGENTS.md. Treat PROJECT_STATE.md as the durable cross-harness authority and use the project-continuity workflow before starting, resuming, handing off, or completing non-trivial work.
`

const codexHooksTemplate = `{
  "description": "Optional Continuity coordination hooks for Codex sessions.",
  "hooks": {
    "SessionStart": [
      {
        "matcher": "startup|resume|clear|compact",
        "hooks": [
          {
            "type": "command",
            "command": "sh \"$(git rev-parse --show-toplevel)/.codex/hooks/continuity.sh\" session-start",
            "statusMessage": "Loading shared Continuity status",
            "timeout": 10,
            "additionalContextLimit": 1800
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": ".*",
        "hooks": [
          {
            "type": "command",
            "command": "sh \"$(git rev-parse --show-toplevel)/.codex/hooks/continuity.sh\" subagent-start",
            "statusMessage": "Loading shared Continuity status",
            "timeout": 10,
            "additionalContextLimit": 1800
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "sh \"$(git rev-parse --show-toplevel)/.codex/hooks/continuity.sh\" user-prompt",
            "statusMessage": "Refreshing shared Continuity status",
            "timeout": 10,
            "additionalContextLimit": 700
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "^(Bash|apply_patch|Edit|Write)$",
        "hooks": [
          {
            "type": "command",
            "command": "sh \"$(git rev-parse --show-toplevel)/.codex/hooks/continuity.sh\" pretool",
            "statusMessage": "Checking worktree claim",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
`

const codexHookScriptTemplate = `#!/bin/sh
set -eu

phase="${1:?hook phase is required}"
root="$(git rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$root" ]; then
  exit 0
fi

if command -v continuity >/dev/null 2>&1; then
  exec continuity codex-hook "$phase" --repo "$root"
fi

# This fallback makes the source repository self-hosting during development.
if [ -f "$root/go.mod" ] && [ -d "$root/cmd/continuity" ]; then
  cd "$root"
  exec go run ./cmd/continuity codex-hook "$phase" --repo "$root"
fi

exit 0
`

const openCodeTemplate = `{
  "$schema": "https://opencode.ai/config.json",
  "instructions": ["AGENTS.md"]
}
`

const skillTemplate = `---
name: project-continuity
description: Resume, coordinate, perform, checkpoint, finalize, or integrate non-trivial engineering work in a repository containing PROJECT_STATE.md. Use at session start, after context loss or compaction, when another local agent may be active, before code edits, and before handoff or completion.
---

# Project continuity

Run continuity context --owner <harness:session-id> and continuity status; Codex may use --session as a convenience alias. Use --scope to select only relevant Context Capsules from the approximately 30k-token durable index. If another owner holds this worktree, do not edit or reinterpret its changes; work read-only, wait, or use a separate worktree. Codex child agents are read-only in their parent checkout; use a separate worktree and claim for a child writer. If state is stale, run continuity reconcile and inspect only listed changes plus mapped code, then sync durable facts.

Before editing, claim an approved autonomous task with continuity start and a narrow --scope. One worktree has one writer; claims reject duplicate tasks and overlapping scopes. Checkpoint material progress, validation, or blockers. Commit, finalize with verification evidence, then merge/hand off and use continuity integrate before declaring the task durable.

Do not replace code inspection with state prose, store raw chats or reasoning, change user-owned product direction, or invent features. Read references/state-schema.md when changing state structure or resolving ownership, freshness, or integration questions.
`

const skillMetadataTemplate = `interface:
  display_name: "Project Continuity"
  short_description: "Coordinate and resume shared project work"
  default_prompt: "Use $project-continuity to safely resume, coordinate, or hand off this repository task."
`

const skillSchemaTemplate = `# State schema

Required front matter: schema, fingerprint, branch, head, status, active_task, and updated_at.

Required sections: Product Intent, Approved Outcomes, Active Work, Ready Queue, Active Decisions, Constraints and Approval Boundaries, Code Map, Context Capsules, Known Pitfalls and Failed Approaches, Verification, Recent Material Changes, and Handoff.

Use task lines in this exact form:

    - [ ] T-001 | Task title | depends: none | autonomous: yes | accepts: Observable completion criteria.

Only autonomous tasks with explicit acceptance criteria can be claimed by a fresh agent. PROJECT_STATE.md stores approximately 30k tokens of bounded durable truth; Context Capsules keep that knowledge path-indexed and continuity context --scope keeps routine prompts selective. Git's local common directory stores live claims, heartbeats, and completions for same-machine worktrees. Use continuity integrate only after the verified worker commit is present in the integration checkout.
`
