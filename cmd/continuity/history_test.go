package main

import (
	"fmt"
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
			Next:        nextWork{Summary: "No task", Action: "Run continuity ready."},
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
