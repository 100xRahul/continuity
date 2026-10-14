package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"continuity/internal/continuity"
)

func TestSyncAgesFactUntilAReviewConfirmsIt(t *testing.T) {
	root := decisionRepo(t)
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceFactCards([]continuity.FactCard{{Status: "fresh", ID: "F-001", Source: "docs/adr/2026-02-10-use-sqlite.md", Paths: []string{"internal/db"}, VerifiedAt: time.Now().UTC(), Evidence: "Store review passed.", Summary: "The store opens one database handle."}}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "commit", "-am", "Add fact")
	writeFile(t, root, "internal/db/store.go", "package db\n\nfunc Open() {}\n")
	runGitCommand(t, root, "commit", "-am", "Change the store")
	if err := checkCommand([]string{"--repo", root}); err == nil {
		t.Fatal("check accepted a fresh fact whose evidence changed")
	}
	if err := syncCommand([]string{"--repo", root}); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, sessionStartContextLimit, nil)
	if err != nil || !strings.Contains(context, "Stale facts, unreliable until reviewed and confirmed with continuity fact confirm: F-001") {
		t.Fatalf("context must flag the stale fact: %v\n%s", err, context)
	}
	runGitCommand(t, root, "commit", "-am", "Sync index")
	if err := factConfirmCommand([]string{"--repo", root, "--id", "F-001", "--evidence", "Reviewed Open after the change."}); err != nil {
		t.Fatal(err)
	}
	if contents, _ := os.ReadFile(filepath.Join(root, continuity.StateFilename)); !strings.Contains(string(contents), "- [fresh] F-001") {
		t.Fatalf("confirmation did not refresh the fact:\n%s", contents)
	}
}
