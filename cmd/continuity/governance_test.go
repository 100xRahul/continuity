package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"continuity/internal/continuity"
)

func TestFinalizationPolicyFailureCheckpointsClaim(t *testing.T) {
	root := governanceRepo(t)
	if err := policyAddCommand([]string{"--repo", root, "--approved", "--id", "P-001", "--scope", "service", "--command", "go test ./does-not-exist", "--reason", "Service verification must pass."}); err != nil {
		t.Fatal(err)
	}
	startGovernanceTask(t, root)
	err := finalizeCommand([]string{"--repo", root, "--owner", "test:worker", "--id", "T-001", "--commit", "HEAD", "--summary", "Attempted finalization.", "--evidence", "focused check: pass"})
	if err == nil || !strings.Contains(err.Error(), "policy P-001 failed") {
		t.Fatalf("failed policy did not gate finalization: %v", err)
	}
	claim, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		t.Fatal(err)
	}
	if claim == nil || !strings.Contains(claim.Risk, "P-001") || !strings.Contains(claim.NextAction, "record a durable blocker") {
		t.Fatalf("policy failure did not checkpoint actionable risk: %#v", claim)
	}
	completed, err := continuity.ListCompletions(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 0 {
		t.Fatalf("policy-gated finalization created completion: %#v", completed)
	}
}

func TestFinalizationRecordsPassingPolicyEvidence(t *testing.T) {
	root := governanceRepo(t)
	if err := policyAddCommand([]string{"--repo", root, "--approved", "--id", "P-001", "--scope", "service", "--command", "go version", "--reason", "Service verification records the Go toolchain check."}); err != nil {
		t.Fatal(err)
	}
	startGovernanceTask(t, root)
	if err := finalizeCommand([]string{"--repo", root, "--owner", "test:worker", "--id", "T-001", "--commit", "HEAD", "--summary", "Finalization passed.", "--evidence", "focused check: pass"}); err != nil {
		t.Fatal(err)
	}
	completed, err := continuity.ListCompletions(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 1 || !strings.Contains(strings.Join(completed[0].Evidence, ";"), "policy P-001: pass") {
		t.Fatalf("passing policy evidence missing: %#v", completed)
	}
}

func TestPolicyChangeAgesFactBackedByPolicyFile(t *testing.T) {
	root := governanceRepo(t)
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceFactCards([]continuity.FactCard{{
		Status:     "fresh",
		ID:         "F-001",
		Source:     "D-001",
		Paths:      []string{continuity.PolicyFilename},
		VerifiedAt: time.Now().UTC(),
		Evidence:   "No policy checks are configured yet.",
		Summary:    "Policy configuration requires review after change.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", continuity.StateFilename)
	runGitCommand(t, root, "commit", "-m", "add policy fact")
	if err := policyAddCommand([]string{"--repo", root, "--approved", "--id", "P-001", "--scope", "service", "--command", "go version", "--reason", "Record policy configuration."}); err != nil {
		t.Fatal(err)
	}
	state, err = continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := state.FactCards()
	if err != nil || len(facts) != 1 || facts[0].Status != "stale" {
		t.Fatalf("policy change did not age its source-linked fact: %#v, %v", facts, err)
	}
}

func TestSyncAgesFactAndClaimRequiresConfirmation(t *testing.T) {
	root := governanceRepo(t)
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceFactCards([]continuity.FactCard{{
		Status:     "fresh",
		ID:         "F-001",
		Source:     "D-001",
		Paths:      []string{"service"},
		VerifiedAt: time.Now().UTC(),
		Evidence:   "Initial service review passed.",
		Summary:    "Service boundary is stable.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", continuity.StateFilename)
	runGitCommand(t, root, "commit", "-m", "add fact card")
	if err := os.MkdirAll(filepath.Join(root, "service"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "service", "main.go"), []byte("package service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", "service/main.go")
	runGitCommand(t, root, "commit", "-m", "change service")
	if err := syncCommand([]string{"--repo", root}); err != nil {
		t.Fatal(err)
	}
	state, err = continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := state.FactCards()
	if err != nil || len(facts) != 1 || facts[0].Status != "stale" {
		t.Fatalf("sync did not age affected fact: %#v, %v", facts, err)
	}
	err = startCommand(governanceStartArgs(root))
	if err == nil || !strings.Contains(err.Error(), "stale fact") {
		t.Fatalf("stale fact did not gate claim: %v", err)
	}
	if err := factConfirmCommand([]string{"--repo", root, "--id", "F-001", "--evidence", "Reviewed service/main.go after change."}); err != nil {
		t.Fatal(err)
	}
	startGovernanceTask(t, root)
}

func TestIgnoredFingerprintSourceFactStillRequiresSynchronization(t *testing.T) {
	root := governanceRepo(t)
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceFactCards([]continuity.FactCard{{
		Status:     "fresh",
		ID:         "F-001",
		Source:     "D-001",
		Paths:      []string{"README.md"},
		VerifiedAt: time.Now().UTC(),
		Evidence:   "Initial README review passed.",
		Summary:    "README guidance is relied on by this task.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", continuity.StateFilename)
	runGitCommand(t, root, "commit", "-m", "add README fact")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("changed instructions\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", "README.md")
	runGitCommand(t, root, "commit", "-m", "change README")
	if err := checkCommand([]string{"--repo", root}); err == nil {
		t.Fatal("check accepted a fresh fact whose ignored evidence source changed")
	}
	args := governanceStartArgs(root)
	for index := range args {
		if args[index] == "service" && index > 0 && args[index-1] == "--scope" {
			args[index] = "README.md"
		}
	}
	err = startCommand(args)
	if err == nil || !strings.Contains(err.Error(), "unsynchronized evidence") {
		t.Fatalf("ignored fingerprint source bypassed fact synchronization: %v", err)
	}
	if err := syncCommand([]string{"--repo", root}); err != nil {
		t.Fatal(err)
	}
	err = startCommand(args)
	if err == nil || !strings.Contains(err.Error(), "stale fact") {
		t.Fatalf("sync did not age ignored fingerprint source fact: %v", err)
	}
}

func TestBlockerPreventsBlindRetryUntilResolved(t *testing.T) {
	root := governanceRepo(t)
	add := []string{"--repo", root, "--id", "B-001", "--task", "T-001", "--scope", "service", "--hypothesis", "Service dependency is unavailable.", "--evidence", "Connection attempt is refused.", "--next-action", "Obtain a reachable test dependency."}
	if err := blockerAddCommand(add); err != nil {
		t.Fatal(err)
	}
	if err := blockerAddCommand(append([]string(nil), add...)); err == nil || !strings.Contains(err.Error(), "do not retry") {
		t.Fatalf("duplicate blocker retry was accepted: %v", err)
	}
	err := startCommand(governanceStartArgs(root))
	if err == nil || !strings.Contains(err.Error(), "open blocker") {
		t.Fatalf("open blocker did not gate claim: %v", err)
	}
	if err := blockerResolveCommand([]string{"--repo", root, "--id", "B-001", "--evidence", "Local test dependency is reachable."}); err != nil {
		t.Fatal(err)
	}
	startGovernanceTask(t, root)
}

func governanceRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test governance lifecycle.", continuity.Snapshot(root), fingerprint)
	queue := "- [ ] T-001 | Govern service work | depends: none | autonomous: yes | accepts: Governance lifecycle is enforced."
	if err := state.ReplaceSection("Ready Queue", queue); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "baseline")
	return root
}

func governanceStartArgs(root string) []string {
	return []string{"--repo", root, "--owner", "test:worker", "--id", "T-001", "--goal", "Govern service work", "--acceptance", "Governance lifecycle is enforced.", "--next-action", "Run focused work.", "--scope", "service"}
}

func startGovernanceTask(t *testing.T, root string) {
	t.Helper()
	if err := startCommand(governanceStartArgs(root)); err != nil {
		t.Fatal(err)
	}
}
