package continuity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossHarnessAdaptersPointToSharedProtocol(t *testing.T) {
	root := moduleRoot(t)
	required := map[string][]string{
		"AGENTS.md":                                  {"PROJECT_PLAN.md", "PROJECT_STATE.md", "continuity context", "continuity start", "one writer"},
		"CLAUDE.md":                                  {"AGENTS.md", "PROJECT_STATE.md"},
		"GEMINI.md":                                  {"AGENTS.md", "PROJECT_STATE.md"},
		".github/copilot-instructions.md":            {"@AGENTS.md", "project-continuity"},
		".cursor/rules/project-continuity.mdc":       {"AGENTS.md", "PROJECT_STATE.md"},
		".codex/hooks.json":                          {"SessionStart", "SubagentStart", "UserPromptSubmit", "PreToolUse", "continuity.sh"},
		".codex/hooks/continuity.sh":                 {"codex-hook", "continuity"},
		"opencode.json":                              {"AGENTS.md"},
		".agents/skills/project-continuity/SKILL.md": {"name: project-continuity", "PROJECT_PLAN.md", "continuity context", "continuity integrate"},
		".agents/skills/project-continuity/references/plan-schema.md":       {"PROJECT_PLAN.md", "active_milestone", "plan approve"},
		".agents/skills/project-continuity/references/governance-schema.md": {"Fact Cards", "Blocker Cards", "CONTINUITY_POLICY.md", "policy add"},
		".agents/worktrunk/README.md":                                       {"Worktrunk", "continuity start", "each writer"},
		".agents/worktrunk/wt.toml.example":                                 {"[pre-start]", "continuity context", "continuity status"},
	}
	for relative, fragments := range required {
		contents, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatalf("read %s: %v", relative, err)
		}
		for _, fragment := range fragments {
			if !strings.Contains(string(contents), fragment) {
				t.Errorf("%s does not contain %q", relative, fragment)
			}
		}
	}
	target, err := os.Readlink(filepath.Join(root, ".claude", "skills"))
	if err != nil {
		t.Fatalf("Claude skill adapter is not a symbolic link: %v", err)
	}
	if target != "../.agents/skills" {
		t.Fatalf("Claude skill adapter target = %q, want ../.agents/skills", target)
	}
}

func TestInstallAdaptersPreservesExistingInstructions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("user policy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := InstallAdapters(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Created) == 0 || len(report.Skipped) == 0 {
		t.Fatalf("unexpected installation report: %#v", report)
	}
	contents, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "user policy\n" {
		t.Fatalf("existing AGENTS.md was overwritten: %q", contents)
	}
	for _, path := range []string{
		"CLAUDE.md",
		"GEMINI.md",
		".github/copilot-instructions.md",
		".cursor/rules/project-continuity.mdc",
		".codex/hooks.json",
		".codex/hooks/continuity.sh",
		"opencode.json",
		".agents/skills/project-continuity/SKILL.md",
		".agents/skills/project-continuity/references/plan-schema.md",
		".agents/skills/project-continuity/references/governance-schema.md",
		".agents/worktrunk/README.md",
		".agents/worktrunk/wt.toml.example",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Errorf("adapter %s was not installed: %v", path, err)
		}
	}
}

func moduleRoot(t *testing.T) string {
	t.Helper()
	directory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			return directory
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			t.Fatal("could not locate module root")
		}
		directory = parent
	}
}
