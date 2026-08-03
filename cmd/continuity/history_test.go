package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"continuity/internal/continuity"
)

func TestUpsertChangeCapsulePreservesArchitectureAndBoundsGeneratedHistory(t *testing.T) {
	state := continuity.NewTemplate("PROJECT_STATE.md", "Test", "Preserve bounded history.", continuity.GitSnapshot{}, "sha256:test")
	if err := state.ReplaceContextCapsules([]continuity.ContextCapsule{{
		Name:    "Architecture",
		Paths:   []string{"internal/continuity"},
		Summary: "Keep durable state portable.",
		Details: "Details: The index is shared across harnesses.",
	}}); err != nil {
		t.Fatal(err)
	}
	for number := 1; number <= 4; number++ {
		id := fmt.Sprintf("T-%03d", number)
		record := changeRecord{
			TaskID:      id,
			Commit:      fmt.Sprintf("commit-%d", number),
			CompletedAt: time.Date(2026, 7, number, 0, 0, 0, 0, time.UTC),
			Intent:      "Complete " + id,
			Result:      "Validated " + id,
			Paths:       []string{"cmd/continuity/" + id + ".go"},
			Evidence:    []string{"go test: pass"},
			Next:        nextWork{Summary: "No task", Action: "Run continuity context."},
			ArchivePath: changeArchivePath(id),
		}
		if err := upsertChangeCapsule(state, record); err != nil {
			t.Fatal(err)
		}
	}
	capsules, err := state.ContextCapsules()
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(capsules))
	for _, capsule := range capsules {
		names = append(names, capsule.Name)
	}
	if got, want := strings.Join(names, ","), "Architecture,Change T-002,Change T-003,Change T-004"; got != want {
		t.Fatalf("capsule retention = %q, want %q", got, want)
	}
}

func TestCapsulePathsUsesProjectWideScopeForWideCrossCuttingChange(t *testing.T) {
	paths := []string{
		"api/a/file.go", "api/b/file.go", "api/c/file.go", "api/d/file.go", "api/e/file.go",
		"api/f/file.go", "api/g/file.go", "api/h/file.go", "api/i/file.go",
	}
	if got, want := strings.Join(capsulePaths(paths), ","), "."; got != want {
		t.Fatalf("wide capsule paths = %q, want %q", got, want)
	}
}

func TestCompactDurableStateKeepsLiveContractsAndNewestEvidence(t *testing.T) {
	root := t.TempDir()
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Keep the active index dense.", continuity.GitSnapshot{}, "sha256:test")
	taskLines := make([]string, 0, 16)
	for number := 1; number <= 15; number++ {
		id := fmt.Sprintf("T-%03d", number)
		taskLines = append(taskLines, continuity.RenderTask(continuity.Task{
			Status:     "x",
			ID:         id,
			Title:      fmt.Sprintf("Completed task %d", number),
			Autonomous: true,
			Acceptance: "The archived result remains inspectable.",
		}))
		if err := writeHistoryFile(root, changeArchivePath(id), "# archived\n"); err != nil {
			t.Fatal(err)
		}
	}
	taskLines = append(taskLines, continuity.RenderTask(continuity.Task{
		Status:     " ",
		ID:         "T-016",
		Title:      "Continue live work",
		DependsOn:  []string{"T-003", "T-015"},
		Autonomous: true,
		Acceptance: "The live task stays actionable.",
	}))
	if err := state.ReplaceSection("Ready Queue", strings.Join(taskLines, "\n")); err != nil {
		t.Fatal(err)
	}
	verification := []string{"- duplicate result: pass", "- duplicate result: pass"}
	changes := []string{}
	for number := 20; number >= 1; number-- {
		verification = append(verification, fmt.Sprintf("- check %d: pass", number))
		changes = append(changes, fmt.Sprintf("- Integrated T-%03d", number))
	}
	if err := state.ReplaceSection("Verification", strings.Join(verification, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceSection("Recent Material Changes", strings.Join(changes, "\n")); err != nil {
		t.Fatal(err)
	}

	if err := compactDurableState(root, state, ""); err != nil {
		t.Fatal(err)
	}
	tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != maxActiveCompletedTasks+1 || tasks[0].ID != "T-004" || tasks[len(tasks)-1].ID != "T-016" {
		t.Fatalf("unexpected compacted queue: %#v", tasks)
	}
	if got := strings.Join(tasks[len(tasks)-1].DependsOn, ","); got != "T-015" {
		t.Fatalf("satisfied archived dependency remained in live contract: %q", got)
	}
	if issues := continuity.AnalyzeTasks(state.Section("Ready Queue")); len(issues) > 0 {
		t.Fatalf("compacted task graph is invalid: %v", issues)
	}
	verificationLines := strings.Split(state.Section("Verification"), "\n")
	if len(verificationLines) != maxActiveVerification || verificationLines[0] != "- duplicate result: pass" || verificationLines[1] != "- check 20: pass" {
		t.Fatalf("verification was not deduplicated and bounded newest-first: %#v", verificationLines)
	}
	changeLines := strings.Split(state.Section("Recent Material Changes"), "\n")
	if len(changeLines) != maxRecentChanges || changeLines[0] != "- Integrated T-020" || changeLines[len(changeLines)-1] != "- Integrated T-009" {
		t.Fatalf("recent changes were not bounded newest-first: %#v", changeLines)
	}
}

func TestCompactDurableStatePreservesTasksForIncompleteOutcome(t *testing.T) {
	root := approvedTaskGraphPlan(t)
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	lines := make([]string, 0, maxActiveCompletedTasks+1)
	for number := 1; number <= maxActiveCompletedTasks+1; number++ {
		id := fmt.Sprintf("T-%03d", number+100)
		lines = append(lines, continuity.RenderTask(continuity.Task{
			Status:     "x",
			ID:         id,
			Title:      "Completed active-outcome task",
			Outcome:    "O-001",
			Scope:      []string{"internal/continuity"},
			Autonomous: true,
			Validation: "go test ./internal/continuity",
			Acceptance: "Focused tests pass.",
		}))
		if err := writeHistoryFile(root, changeArchivePath(id), "# archived\n"); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.ReplaceSection("Ready Queue", strings.Join(lines, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := compactDurableState(root, state, ""); err != nil {
		t.Fatal(err)
	}
	tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != maxActiveCompletedTasks+1 {
		t.Fatalf("incomplete-outcome evidence was compacted before outcome verification: %d tasks remain", len(tasks))
	}
}
