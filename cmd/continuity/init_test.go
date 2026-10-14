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
	if state.Meta["schema"] != continuity.CurrentStateSchema {
		t.Fatalf("new project state has schema %q", state.Meta["schema"])
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
		".claude/settings.json",
		"opencode.json",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("missing initialized file %s: %v", path, err)
		}
	}
}
