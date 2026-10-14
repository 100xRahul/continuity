package main

import (
	"os"
	"path/filepath"
	"testing"

	"continuity/internal/continuity"
)

func TestCheckRejectsStaleSourceState(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "feature.go")
	if err := os.WriteFile(source, []byte("package feature\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("package feature\n\nfunc Changed() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkCommand([]string{"--repo", root}); err == nil {
		t.Fatal("expected stale state check to fail")
	}
}
