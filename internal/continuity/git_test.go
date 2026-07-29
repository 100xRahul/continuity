package continuity

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewAnchorsAndChangedSinceState(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "src", "parser.go")
	if err := os.WriteFile(source, []byte("package src\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := NewTemplate(filepath.Join(root, StateFilename), "Test", "Test review anchors.", Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Active Decisions", "- D-001 — Preserve the parser boundary. Evidence: `src/parser.go`."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	configureTestGit(t, root)
	runTestGit(t, root, "add", ".")
	runTestGit(t, root, "commit", "-m", "baseline state")

	if err := os.WriteFile(source, []byte("package src\n\nfunc Parse() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(ReviewAnchors(root, state), ","); got != "src/parser.go" {
		t.Fatalf("review anchors = %q, want src/parser.go", got)
	}
	runTestGit(t, root, "add", "src/parser.go")
	runTestGit(t, root, "commit", "-m", "change parser")
	changed, err := ChangedSinceState(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(changed, "\n"); !strings.Contains(got, "src/parser.go") {
		t.Fatalf("changes since state = %q", got)
	}
}

func TestFingerprintUsesGitFilesAndHonorsIgnoreRules(t *testing.T) {
	root := t.TempDir()
	if _, err := EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/continuity\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "continuity"), []byte("first binary build"), 0o755); err != nil {
		t.Fatal(err)
	}
	before, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "continuity"), []byte("second binary build"), 0o755); err != nil {
		t.Fatal(err)
	}
	unchanged, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if before != unchanged {
		t.Fatal("ignored root binary changed the source fingerprint")
	}
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	if after == unchanged {
		t.Fatal("source change did not change Git-aware fingerprint")
	}
}

func configureTestGit(t *testing.T, root string) {
	t.Helper()
	runTestGit(t, root, "config", "user.name", "Continuity Test")
	runTestGit(t, root, "config", "user.email", "continuity@example.test")
}

func runTestGit(t *testing.T, root string, args ...string) {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
}
