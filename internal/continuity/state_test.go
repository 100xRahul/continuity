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

func TestStateValidationRejectsDuplicateSections(t *testing.T) {
	state := NewState("PROJECT_STATE.md", map[string]string{
		"schema": "1", "fingerprint": "sha256:test", "branch": "main", "head": "unborn", "status": "ready", "active_task": "none", "updated_at": "2026-07-30T00:00:00Z",
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
