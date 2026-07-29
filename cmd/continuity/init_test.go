package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitCreatesFreshScaffoldedRepository(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--project", "Test", "--objective", "Prove init."}); err != nil {
		t.Fatal(err)
	}
	if err := checkCommand([]string{"--repo", root}); err != nil {
		t.Fatalf("newly initialized repository is not fresh: %v", err)
	}
	for _, path := range []string{
		"PROJECT_STATE.md",
		"AGENTS.md",
		".agents/skills/project-continuity/SKILL.md",
		".github/copilot-instructions.md",
		".cursor/rules/project-continuity.mdc",
		"opencode.json",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("missing initialized file %s: %v", path, err)
		}
	}
}
