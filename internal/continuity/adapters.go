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
		"AGENTS.md":                            agentsTemplate,
		"CLAUDE.md":                            claudeTemplate,
		"GEMINI.md":                            geminiTemplate,
		".github/copilot-instructions.md":      copilotTemplate,
		".cursor/rules/project-continuity.mdc": cursorTemplate,
		"opencode.json":                        openCodeTemplate,
		".agents/skills/project-continuity/SKILL.md":                   skillTemplate,
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

This repository uses PROJECT_STATE.md as durable shared project state. Treat chat history and harness-specific memory as disposable conveniences, not authority.

Before non-trivial work, use the project-continuity skill when available, run continuity brief, and read PROJECT_STATE.md. If state is stale or the worktree is dirty, run continuity reconcile and inspect only listed changes plus mapped relevant code. Resume Active Work; otherwise run continuity ready and select only an explicit autonomous task with acceptance criteria.

Before editing, run continuity start unless work is already active. Checkpoint material progress or blockers. Before completion, record verification and use continuity finalize, then continuity check.

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

const openCodeTemplate = `{
  "$schema": "https://opencode.ai/config.json",
  "instructions": ["AGENTS.md"]
}
`

const skillTemplate = `---
name: project-continuity
description: Resume, perform, checkpoint, or finalize non-trivial engineering work in a repository containing PROJECT_STATE.md. Use at session start, after context loss or compaction, when changing harnesses, before code edits, and before handoff or completion.
---

# Project continuity

Run continuity brief and read the emitted state. If stale or dirty, run continuity reconcile and inspect only listed changes plus code mapped to active work. Resume Active Work; otherwise run continuity ready and select only an approved autonomous task with acceptance criteria.

Before editing, run continuity start unless the task is already active. Honor strict, soft, or exploratory execution mode. Checkpoint material progress, validation, or blockers. Finalize only after recording verification, then run continuity check.

Do not replace code inspection with state prose, store raw chats or reasoning, change user-owned product direction, or invent features. Read references/state-schema.md when changing state structure or resolving ownership/freshness questions.
`

const skillMetadataTemplate = `interface:
  display_name: "Project Continuity"
  short_description: "Resume work from durable project state"
  default_prompt: "Use $project-continuity to resume or hand off this repository task."
`

const skillSchemaTemplate = `# State schema

Required front matter: schema, fingerprint, branch, head, status, active_task, and updated_at.

Required sections: Product Intent, Approved Outcomes, Active Work, Ready Queue, Active Decisions, Constraints and Approval Boundaries, Code Map, Known Pitfalls and Failed Approaches, Verification, Recent Material Changes, and Handoff.

Use task lines in this exact form:

    - [ ] T-001 | Task title | depends: none | autonomous: yes | accepts: Observable completion criteria.

Only autonomous tasks with explicit acceptance criteria can be selected by a fresh agent. Git stores history; PROJECT_STATE.md stores bounded current truth.
`
