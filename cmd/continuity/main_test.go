package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"continuity/internal/continuity"
)

func TestLifecycleCommands(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test lifecycle.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | Prove lifecycle | depends: none | autonomous: yes | accepts: State reaches ready after finalization."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	if err := startCommand([]string{"--repo", root, "--id", "T-001", "--goal", "Prove lifecycle", "--acceptance", "State reaches ready after finalization.", "--next-action", "Write the checkpoint."}); err != nil {
		t.Fatal(err)
	}
	if err := checkpointCommand([]string{"--repo", root, "--summary", "Started and inspected state.", "--next-action", "Finalize the task."}); err != nil {
		t.Fatal(err)
	}
	if err := finalizeCommand([]string{"--repo", root, "--summary", "Lifecycle command test passed."}); err != nil {
		t.Fatal(err)
	}

	finished, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if finished.Meta["status"] != "ready" || finished.Meta["active_task"] != "none" {
		t.Fatalf("unexpected final state: status=%q active=%q", finished.Meta["status"], finished.Meta["active_task"])
	}
	if !strings.Contains(finished.Section("Ready Queue"), "[x] T-001") {
		t.Fatalf("task was not marked complete: %s", finished.Section("Ready Queue"))
	}
	if !strings.Contains(finished.Section("Recent Material Changes"), "Completed T-001") {
		t.Fatalf("completion was not recorded: %s", finished.Section("Recent Material Changes"))
	}
	if _, err := os.Stat(filepath.Join(root, continuity.StateFilename)); err != nil {
		t.Fatal(err)
	}
}

func TestStartRejectsTaskWithUnfinishedDependency(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test dependencies.", continuity.Snapshot(root), fingerprint)
	queue := strings.Join([]string{
		"- [ ] T-001 | First | depends: none | autonomous: yes | accepts: First is complete.",
		"- [ ] T-002 | Second | depends: T-001 | autonomous: yes | accepts: Second is complete.",
	}, "\n")
	if err := state.ReplaceSection("Ready Queue", queue); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}

	err = startCommand([]string{"--repo", root, "--id", "T-002", "--goal", "Second", "--acceptance", "Second is complete.", "--next-action", "Try to start."})
	if err == nil || !strings.Contains(err.Error(), "unfinished dependency") {
		t.Fatalf("expected unfinished dependency error, got %v", err)
	}
}
