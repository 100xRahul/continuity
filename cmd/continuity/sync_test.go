package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"continuity/internal/continuity"
)

func TestSyncUpdatesGitFactsWithoutChangingTaskState(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test sync.", continuity.Snapshot(root), fingerprint)
	state.SetMeta("status", "working")
	state.SetMeta("active_task", "T-001")
	if err := state.ReplaceSection("Active Work", "Task: T-001\nStatus: in_progress\nMode: strict\nGoal: preserve active work\nAcceptance criteria: task state remains active\nImmediate next action: synchronize\nEdit scope: none\nValidation: none"); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "baseline")
	head := strings.TrimSpace(runGitCommand(t, root, "rev-parse", "HEAD"))

	if err := syncCommand([]string{"--repo", root}); err != nil {
		t.Fatal(err)
	}
	synced, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if synced.Meta["head"] != head || synced.Meta["branch"] != "main" {
		t.Fatalf("unexpected Git metadata: %#v", synced.Meta)
	}
	if synced.Meta["status"] != "working" || synced.Meta["active_task"] != "T-001" {
		t.Fatalf("sync changed task state: %#v", synced.Meta)
	}
}

func runGitCommand(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}
