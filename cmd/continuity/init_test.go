package main

import (
	"os"
	"path/filepath"
	"testing"

	"continuity/internal/continuity"
)

func TestInitCreatesFreshScaffoldedRepository(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--project", "Test", "--objective", "Prove init."}); err != nil {
		t.Fatal(err)
	}
	if err := checkCommand([]string{"--repo", root}); err != nil {
		t.Fatalf("newly initialized repository is not fresh: %v", err)
	}
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if state.Meta["plan_status"] != "draft" || state.Meta["plan_active_milestone"] != "none" {
		t.Fatalf("new project did not project draft plan state: %#v", state.Meta)
	}
	if tasks, err := continuity.ParseTasks(state.Section("Ready Queue")); err != nil || len(tasks) != 0 {
		t.Fatalf("new draft should not reserve a task ID: %#v, %v", tasks, err)
	}
	for _, path := range []string{
		"PROJECT_STATE.md",
		"PROJECT_PLAN.md",
		"AGENTS.md",
		".agents/skills/project-continuity/SKILL.md",
		".agents/skills/project-continuity/references/plan-schema.md",
		".github/copilot-instructions.md",
		".cursor/rules/project-continuity.mdc",
		".codex/hooks.json",
		".agents/hooks/continuity.sh",
		".agents/skills/record-decisions/SKILL.md",
		"docs/adr/README.md",
		"docs/adr/template.md",
		".agents/worktrunk/README.md",
		".agents/worktrunk/wt.toml.example",
		"opencode.json",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("missing initialized file %s: %v", path, err)
		}
	}
}
