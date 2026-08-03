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

func TestInstallAdaptersConnectsExistingInstructionsWithoutOverwriting(t *testing.T) {
	root := t.TempDir()
	existingText := map[string]string{
		"AGENTS.md":                       "agent policy\n",
		"CLAUDE.md":                       "claude policy\n",
		"GEMINI.md":                       "gemini policy\n",
		".github/copilot-instructions.md": "copilot policy\n",
	}
	for path, content := range existingText {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	existingJSON := map[string]string{
		".codex/hooks.json": `{"custom":"keep","hooks":{"SessionStart":[{"hooks":[{"type":"command","command":"custom-hook"}]}]}}`,
		"opencode.json":     `{"theme":"keep","instructions":["EXISTING.md"]}`,
	}
	for path, content := range existingJSON {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	report, err := InstallAdapters(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Created) == 0 || len(report.Updated) != len(existingText)+len(existingJSON) || len(report.Attention) != 0 {
		t.Fatalf("unexpected installation report: %#v", report)
	}
	for path, original := range existingText {
		contents, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(contents)
		if !strings.HasPrefix(text, original) {
			t.Fatalf("existing %s content was not preserved: %q", path, contents)
		}
		if !strings.Contains(text, continuityBridgeStart) || !strings.Contains(text, "PROJECT_STATE.md") {
			t.Fatalf("existing %s was not connected to Continuity: %q", path, contents)
		}
	}
	for path, original := range existingJSON {
		contents, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(contents)
		if path == ".codex/hooks.json" && (!strings.Contains(text, `"custom": "keep"`) || !strings.Contains(text, "custom-hook") || !strings.Contains(text, "continuity.sh")) {
			t.Fatalf("Codex hooks were not merged safely; original=%s updated=%s", original, text)
		}
		if path == "opencode.json" && (!strings.Contains(text, `"theme": "keep"`) || !strings.Contains(text, "EXISTING.md") || !strings.Contains(text, "AGENTS.md")) {
			t.Fatalf("OpenCode instructions were not merged safely; original=%s updated=%s", original, text)
		}
		info, statErr := os.Stat(filepath.Join(root, path))
		if statErr != nil {
			t.Fatal(statErr)
		}
		if info.Mode().Perm() != 0o640 {
			t.Fatalf("%s mode changed to %o", path, info.Mode().Perm())
		}
	}
	for _, path := range []string{
		".cursor/rules/project-continuity.mdc",
		".codex/hooks/continuity.sh",
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
	second, err := InstallAdapters(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Updated) != 0 {
		t.Fatalf("second install should be idempotent: %#v", second)
	}
	for path := range existingText {
		contents, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if strings.Count(string(contents), continuityBridgeStart) != 1 {
			t.Fatalf("continuity bridge in %s was duplicated: %q", path, contents)
		}
	}
	for path := range existingJSON {
		contents, readErr := os.ReadFile(filepath.Join(root, path))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if path == ".codex/hooks.json" && strings.Count(string(contents), "continuity.sh") != 4 {
			t.Fatalf("Continuity hooks in %s were duplicated: %s", path, contents)
		}
		if path == "opencode.json" && strings.Count(string(contents), "AGENTS.md") != 1 {
			t.Fatalf("Continuity instruction in %s was duplicated: %s", path, contents)
		}
	}
}

func TestInstallAdaptersDoesNotFollowExistingInstructionSymlink(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "AGENTS.md")
	if err := os.WriteFile(external, []byte("external policy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatal(err)
	}
	report, err := InstallAdapters(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 0 {
		t.Fatalf("symlinked instruction file was reported as updated: %#v", report)
	}
	contents, err := os.ReadFile(external)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "external policy\n" {
		t.Fatalf("external symlink target was changed: %q", contents)
	}
}

func TestInstallAdaptersPreservesUnsupportedStructuredConfig(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "opencode.json")
	if err := os.WriteFile(path, []byte("null\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	report, err := InstallAdapters(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Attention) != 1 || !strings.Contains(report.Attention[0], "opencode.json") {
		t.Fatalf("unsupported config did not produce a specific warning: %#v", report)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "null\n" {
		t.Fatalf("unsupported config was changed: %q", contents)
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
