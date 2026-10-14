package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"continuity/internal/continuity"
)

func TestRecallFindsCurrentDecisionsPitfallsAndCommits(t *testing.T) {
	root := decisionRepo(t)

	answer := captureStdout(t, func() error { return recallCommand([]string{"--repo", root, "which", "database", "stores", "data"}) })
	first, _, _ := strings.Cut(answer, "\n")
	if !strings.Contains(first, "[decision] Use SQLite for local storage") {
		t.Fatalf("the current decision must rank first:\n%s", answer)
	}
	if !strings.Contains(answer, "[superseded decision] Use Postgres for storage — docs/adr/2026-01-05-use-postgres.md (superseded by docs/adr/2026-02-10-use-sqlite.md)") {
		t.Fatalf("an old decision must be labelled with its replacement:\n%s", answer)
	}

	linked := captureStdout(t, func() error { return recallCommand([]string{"--repo", root, "--path", "internal/db/store.go"}) })
	for _, want := range []string{
		"Decisions recorded for internal/db/store.go:\n- Use SQLite for local storage → docs/adr/2026-02-10-use-sqlite.md",
		"Known pitfalls for internal/db/store.go:\n- Do not open the database file twice",
		"Recent commits:\n- ",
		"Add the storage layer",
	} {
		if !strings.Contains(linked, want) {
			t.Errorf("path recall lacks %q:\n%s", want, linked)
		}
	}
	if strings.Contains(linked, "Postgres") {
		t.Fatalf("path recall must not present a superseded decision:\n%s", linked)
	}
}

// decisionRepo builds a committed repository with a superseded and a current
// storage decision, a path-linked pitfall, and a commit touching that path.
func decisionRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "internal/db/store.go", "package db\n")
	writeFile(t, root, "docs/adr/2026-01-05-use-postgres.md", "# Use Postgres for storage\n\n- Date: 2026-01-05\n- Paths: internal/db\n\n## Decision\n\nStore data in Postgres.\n")
	writeFile(t, root, "docs/adr/2026-02-10-use-sqlite.md", "# Use SQLite for local storage\n\n- Date: 2026-02-10\n- Paths: internal/db\n- Supersedes: 2026-01-05-use-postgres.md\n\n## Decision\n\nStore data in one SQLite database file so the tool deploys as one binary.\n")
	writeFile(t, root, "docs/adr/2026-01-01-snake-case.md", "# Keep API fields in snake_case\n\n- Date: 2026-01-01\n- Paths: .\n")
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, continuity.PlanFilename, "---\nschema: 2\n---\n\n# Test plan\n\n## Intent\n\nStore data locally.\n\n## Milestones\n\n- [>] M-001 | title: Local storage | accepts: Data survives a restart.\n\n## Outcomes\n\n- [x] O-001 | milestone: M-001 | title: Choose the database | accepts: A decision record exists.\n- [ ] O-002 | milestone: M-001 | title: Persist sessions | accepts: A restarted process reads earlier sessions.\n")
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection(continuity.PitfallsSection, "- Do not open the database file twice; the second handle corrupts writes. Evidence: `internal/db/store.go`."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "Add the storage layer", "-m", "SQLite keeps deployment to one file.")
	return root
}

func writeFile(t *testing.T, root, relative, contents string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func captureStdout(t *testing.T, run func() error) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = writer
	runErr := run()
	os.Stdout = original
	writer.Close()
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("command failed: %v\n%s", runErr, output)
	}
	return string(output)
}
