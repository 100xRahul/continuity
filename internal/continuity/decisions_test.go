package continuity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDecision(t *testing.T, root, name, contents string) {
	t.Helper()
	directory := filepath.Join(root, filepath.FromSlash(DecisionDir))
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadDecisionsKeepsOnlyTheReplacementCurrent(t *testing.T) {
	root := t.TempDir()
	writeDecision(t, root, "2026-01-05-use-postgres.md", "# Use Postgres for storage\n\n- Date: 2026-01-05\n- Paths: internal/db\n\n## Decision\n\nPostgres.\n")
	writeDecision(t, root, "2026-02-10-use-sqlite.md", "# Use SQLite for storage\n\n- Date: 2026-02-10\n- Paths: internal/db\n- Supersedes: [Use Postgres](2026-01-05-use-postgres.md)\n\n## Decision\n\nOne file deploys.\n")
	writeDecision(t, root, "2026-01-01-snake-case-api.md", "# Keep API fields in snake_case\n\n- Date: 2026-01-01\n- Paths: .\n")
	writeDecision(t, root, "README.md", "# Guide\n")

	decisions, problems, err := LoadDecisions(root)
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v", problems, err)
	}
	current := []string{}
	for _, decision := range CurrentDecisions(decisions) {
		current = append(current, decision.Title)
	}
	if got, want := strings.Join(current, "; "), "Keep API fields in snake_case; Use SQLite for storage"; got != want {
		t.Fatalf("current decisions = %q, want %q", got, want)
	}
	for _, decision := range decisions {
		if decision.Title == "Use Postgres for storage" && strings.Join(decision.SupersededBy, ",") != "docs/adr/2026-02-10-use-sqlite.md" {
			t.Fatalf("superseded record lost its replacement link: %#v", decision)
		}
	}
	linked := DecisionsForPaths(decisions, []string{"internal/db/store.go"})
	if len(linked) != 1 || linked[0].Title != "Use SQLite for storage" {
		t.Fatalf("path-linked decisions must be current and path-specific: %#v", linked)
	}
}

func TestLoadDecisionsReportsEveryBrokenRecordAndKeepsValidOnes(t *testing.T) {
	root := t.TempDir()
	writeDecision(t, root, "2026-03-01-valid.md", "# Valid decision\n\n- Date: 2026-03-01\n- Paths: cmd\n")
	writeDecision(t, root, "2026-03-02-no-paths.md", "# Missing paths\n\n- Date: 2026-03-02\n")
	writeDecision(t, root, "notes.md", "# Not a record name\n\n- Date: 2026-03-03\n- Paths: .\n")
	writeDecision(t, root, "2026-03-04-dangling.md", "# Dangling link\n\n- Date: 2026-03-04\n- Paths: cmd\n- Supersedes: 2026-01-01-missing.md\n")
	writeDecision(t, root, "2026-03-05-loop-a.md", "# Loop A\n\n- Date: 2026-03-05\n- Paths: cmd\n- Supersedes: 2026-03-06-loop-b.md\n")
	writeDecision(t, root, "2026-03-06-loop-b.md", "# Loop B\n\n- Date: 2026-03-06\n- Paths: cmd\n- Supersedes: 2026-03-05-loop-a.md\n")

	decisions, problems, err := LoadDecisions(root)
	if err != nil {
		t.Fatal(err)
	}
	report := strings.Join(problems, "\n")
	for _, want := range []string{"2026-03-02-no-paths.md: missing '- Paths", "notes.md: name must be", "2026-03-04-dangling.md: supersedes docs/adr/2026-01-01-missing.md", "cycle"} {
		if !strings.Contains(report, want) {
			t.Errorf("problems do not mention %q:\n%s", want, report)
		}
	}
	valid := false
	for _, decision := range CurrentDecisions(decisions) {
		valid = valid || decision.Title == "Valid decision"
	}
	if !valid {
		t.Fatalf("a valid record was hidden by broken neighbours: %#v", decisions)
	}
}
