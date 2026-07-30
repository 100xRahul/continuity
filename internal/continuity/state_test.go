package continuity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStateRoundTripAndSectionReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), StateFilename)
	state := NewTemplate(path, "Example", "Build a reliable workflow.", GitSnapshot{Branch: "main"}, "sha256:before")
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if result := loaded.Validate(DefaultStateBudget); !result.OK() {
		t.Fatalf("unexpected validation errors: %v", result.Errors)
	}
	if err := loaded.ReplaceSection("Handoff", "Last completed: test\nImmediate restart action: continue"); err != nil {
		t.Fatal(err)
	}
	if got := loaded.Section("Handoff"); !strings.Contains(got, "Last completed: test") {
		t.Fatalf("unexpected handoff: %q", got)
	}
	if err := loaded.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Section("Handoff"); !strings.Contains(got, "Immediate restart action: continue") {
		t.Fatalf("replacement did not persist: %q", got)
	}
}

func TestFingerprintExcludesStateAndAdapters(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, StateFilename), []byte("state one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agents", "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agents", "skills", "ignored.md"), []byte("adapter one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "opencode.json"), []byte("{\"instructions\":[\"AGENTS.md\"]}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, StateFilename), []byte("state two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	unchanged, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != unchanged {
		t.Fatal("state file changed the source fingerprint")
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("source change did not change fingerprint")
	}
}

func TestEvidencePathsUsesOnlyDurablePathAnchors(t *testing.T) {
	state := NewState("PROJECT_STATE.md", map[string]string{}, "# Example\n\n"+
		"## Active Decisions\n\n"+
		"- D-001 — Preserve the parser boundary. Evidence: `internal/continuity/state.go`.\n\n"+
		"## Code Map\n\n"+
		"- CLI — `cmd/continuity/`.\n\n"+
		"## Known Pitfalls and Failed Approaches\n\n"+
		"- Avoid `go test ./...` as an evidence path.\n\n"+
		"## Active Work\n\n"+
		"Edit scope: `internal/continuity`.\n")
	got := strings.Join(state.EvidencePaths(), ",")
	want := "cmd/continuity,internal/continuity,internal/continuity/state.go"
	if got != want {
		t.Fatalf("evidence paths = %q, want %q", got, want)
	}
}

func TestContextCapsulesSelectRelevantScopes(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Example", "Test capsule selection.", GitSnapshot{}, "sha256:test")
	if err := state.ReplaceSection("Context Capsules", strings.Join([]string{
		"### Project-wide orientation",
		"Paths: .",
		"Summary: Applies to every task.",
		"Details: Global constraints.",
		"",
		"### State engine",
		"Paths: internal/continuity",
		"Summary: Parsing and durable-index validation.",
		"Details: Inspect for state changes.",
		"",
		"### CLI commands",
		"Paths: cmd/continuity",
		"Summary: User-facing lifecycle commands.",
		"Details: Inspect for CLI changes.",
	}, "\n")); err != nil {
		t.Fatal(err)
	}
	capsules, err := state.ContextCapsules()
	if err != nil {
		t.Fatal(err)
	}
	selected := SelectContextCapsules(capsules, []string{"internal/continuity/state.go"})
	if len(selected) != 2 || selected[0].Name != "State engine" || selected[1].Name != "Project-wide orientation" {
		t.Fatalf("selected capsules = %#v", selected)
	}
	global := SelectContextCapsules(capsules, nil)
	if len(global) != 1 || global[0].Name != "Project-wide orientation" {
		t.Fatalf("unscoped capsules = %#v", global)
	}
	if result := state.Validate(DefaultStateBudget); !result.OK() {
		t.Fatalf("valid capsules rejected: %v", result.Errors)
	}
}

func TestContextCapsulesRejectMalformedPaths(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Example", "Test malformed capsule.", GitSnapshot{}, "sha256:test")
	if err := state.ReplaceSection("Context Capsules", "### Escaping capsule\nPaths: ../secret\nSummary: Invalid path."); err != nil {
		t.Fatal(err)
	}
	result := state.Validate(DefaultStateBudget)
	if result.OK() || !strings.Contains(strings.Join(result.Errors, "\n"), "escapes the repository") {
		t.Fatalf("malformed capsule accepted: %#v", result)
	}
}

func TestReplaceContextCapsulesPreservesStructuredRecords(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Example", "Test capsule replacement.", GitSnapshot{}, "sha256:test")
	capsules := []ContextCapsule{
		{Name: "Architecture", Paths: []string{"internal/continuity"}, Summary: "Core state behavior.", Details: "Details: Keep the parser bounded."},
		{Name: "Change T-001", Paths: []string{"cmd/continuity"}, Summary: "Intent: Add lifecycle command.", Details: "Details: Commit: abc123\nChanged paths: cmd/continuity/main.go\nValidation: go test: pass\nNext: T-002"},
	}
	if err := state.ReplaceContextCapsules(capsules); err != nil {
		t.Fatal(err)
	}
	parsed, err := state.ContextCapsules()
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed[1].Name != "Change T-001" || !strings.Contains(parsed[1].Details, "Validation: go test: pass") {
		t.Fatalf("capsules did not round trip: %#v", parsed)
	}
}

func TestDefaultStateBudgetApproximatesThirtyThousandTokens(t *testing.T) {
	if DefaultStateBudget != ApproximateStateTokenBudget*ApproximateBytesPerToken {
		t.Fatalf("state budget = %d, want %d", DefaultStateBudget, ApproximateStateTokenBudget*ApproximateBytesPerToken)
	}
	if ApproximateStateTokenBudget != 30_000 {
		t.Fatalf("token budget = %d, want 30000", ApproximateStateTokenBudget)
	}
}

func TestDefaultStateBudgetSupportsLargeDurableCapsules(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Example", "Test durable capacity.", GitSnapshot{}, "sha256:test")
	if err := state.ReplaceSection("Context Capsules", "### Large architecture record\nPaths: internal\nSummary: A bounded large durable record.\nDetails: "+strings.Repeat("x", 100_000)); err != nil {
		t.Fatal(err)
	}
	if result := state.Validate(DefaultStateBudget); !result.OK() {
		t.Fatalf("large in-budget capsule rejected: %v", result.Errors)
	}
	if err := state.ReplaceSection("Context Capsules", "### Too large\nPaths: internal\nSummary: Exceeds the durable budget.\nDetails: "+strings.Repeat("x", DefaultStateBudget)); err != nil {
		t.Fatal(err)
	}
	if result := state.Validate(DefaultStateBudget); result.OK() {
		t.Fatal("over-budget capsule was accepted")
	}
}

func TestStateValidationRequiresCurrentSchema(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Example", "Test current schema.", GitSnapshot{}, "sha256:test")
	state.SetMeta("schema", "1")
	result := state.Validate(DefaultStateBudget)
	if result.OK() || !strings.Contains(strings.Join(result.Errors, "\n"), "expected "+CurrentStateSchema) {
		t.Fatalf("unsupported schema was accepted: %#v", result)
	}
}

func TestStateValidationRejectsDuplicateSections(t *testing.T) {
	state := NewState("PROJECT_STATE.md", map[string]string{
		"schema": CurrentStateSchema, "fingerprint": "sha256:test", "branch": "main", "head": "unborn", "status": "ready", "active_task": "none", "updated_at": "2026-07-30T00:00:00Z",
	}, "## Product Intent\n\nfirst\n\n## Product Intent\n\nsecond\n")
	result := state.Validate(DefaultStateBudget)
	if result.OK() || !strings.Contains(strings.Join(result.Errors, "\n"), "appears more than once") {
		t.Fatalf("duplicate section was accepted: %#v", result)
	}
}

func TestLoadStateRejectsDuplicateFrontMatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), StateFilename)
	contents := "---\nschema: 1\nschema: 1\n---\n\n# Test\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadState(path)
	if err == nil || !strings.Contains(err.Error(), "duplicate front matter") {
		t.Fatalf("duplicate front matter was accepted: %v", err)
	}
}
