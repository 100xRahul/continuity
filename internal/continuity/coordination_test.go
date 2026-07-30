package continuity

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestClaimsAreExclusiveAcrossWorktrees(t *testing.T) {
	root, peer := testWorktrees(t)
	first, err := ClaimTask(root, ClaimRequest{
		TaskID:   "T-001",
		Owner:    "codex:first",
		Worktree: root,
		Branch:   "main",
		Scope:    []string{"internal/continuity"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if first.Worktree == "" || first.Nonce == "" {
		t.Fatalf("claim was not fully populated: %#v", first)
	}
	_, err = ClaimTask(peer, ClaimRequest{
		TaskID:   "T-001",
		Owner:    "codex:second",
		Worktree: peer,
		Branch:   "peer",
		Scope:    []string{"cmd/continuity"},
	})
	var conflict *ClaimConflictError
	if !errors.As(err, &conflict) || conflict.Reason != "task is already claimed" {
		t.Fatalf("expected task conflict, got %v", err)
	}
	_, err = ClaimTask(peer, ClaimRequest{
		TaskID:   "T-002",
		Owner:    "codex:second",
		Worktree: peer,
		Branch:   "peer",
		Scope:    []string{"internal/continuity/state.go"},
	})
	if !errors.As(err, &conflict) || conflict.Reason != "edit scope overlaps an active claim" {
		t.Fatalf("expected scope conflict, got %v", err)
	}
	_, err = ClaimTask(peer, ClaimRequest{
		TaskID:   "T-003",
		Owner:    "codex:second",
		Worktree: peer,
		Branch:   "peer",
		Scope:    []string{"cmd/continuity"},
	})
	if err != nil {
		t.Fatalf("independent worktree scope should be claimable: %v", err)
	}
}

func TestClaimRejectsSecondWriterInSameWorktree(t *testing.T) {
	root, _ := testWorktrees(t)
	if _, err := ClaimTask(root, ClaimRequest{TaskID: "T-001", Owner: "codex:first", Worktree: root, Scope: []string{"cmd"}}); err != nil {
		t.Fatal(err)
	}
	_, err := ClaimTask(root, ClaimRequest{TaskID: "T-002", Owner: "codex:second", Worktree: root, Scope: []string{"internal"}})
	var conflict *ClaimConflictError
	if !errors.As(err, &conflict) || conflict.Reason != "worktree already has an active writer" {
		t.Fatalf("expected worktree conflict, got %v", err)
	}
}

func TestClaimRejectsSecondWriterOnSameBranch(t *testing.T) {
	root, peer := testWorktrees(t)
	if _, err := ClaimTask(root, ClaimRequest{TaskID: "T-001", Owner: "codex:first", Worktree: root, Branch: "agent/auth", Scope: []string{"internal/auth"}}); err != nil {
		t.Fatal(err)
	}
	_, err := ClaimTask(peer, ClaimRequest{TaskID: "T-002", Owner: "codex:second", Worktree: peer, Branch: "agent/auth", Scope: []string{"web"}})
	var conflict *ClaimConflictError
	if !errors.As(err, &conflict) || conflict.Reason != "branch already has an active writer" {
		t.Fatalf("expected branch conflict, got %v", err)
	}
}

func TestConcurrentClaimsHaveOneWinner(t *testing.T) {
	root, _ := testWorktrees(t)
	start := make(chan struct{})
	results := make(chan error, 12)
	var wait sync.WaitGroup
	for index := 0; index < cap(results); index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			_, err := ClaimTask(root, ClaimRequest{
				TaskID:   "T-001",
				Owner:    "codex:parallel",
				Worktree: root,
				Scope:    []string{"cmd"},
			})
			results <- err
		}(index)
	}
	close(start)
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("claim successes = %d, want 1", successes)
	}
}

func TestCheckpointAndCompletionLeaveSharedTerminalRecord(t *testing.T) {
	root, _ := testWorktrees(t)
	if _, err := ClaimTask(root, ClaimRequest{TaskID: "T-001", Owner: "codex:worker", Worktree: root, Scope: []string{"cmd"}, NextAction: "implement"}); err != nil {
		t.Fatal(err)
	}
	claim, err := CheckpointClaim(root, CheckpointRequest{
		TaskID:     "T-001",
		Owner:      "codex:worker",
		Worktree:   root,
		Summary:    "Implemented the claim store.",
		NextAction: "Run tests.",
		Risk:       "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if claim.Progress != "Implemented the claim store." || claim.NextAction != "Run tests." {
		t.Fatalf("checkpoint not persisted: %#v", claim)
	}
	completion, err := CompleteClaim(root, CompletionRequest{
		TaskID:   "T-001",
		Owner:    "codex:worker",
		Worktree: root,
		Commit:   "abc123",
		Summary:  "Claim store complete.",
		Evidence: []string{"go test ./...: pass"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if completion.Commit != "abc123" {
		t.Fatalf("unexpected completion: %#v", completion)
	}
	claims, err := ListClaims(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 0 {
		t.Fatalf("active claims = %#v, want none", claims)
	}
	completed, err := ListCompletions(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 1 || completed[0].TaskID != "T-001" {
		t.Fatalf("completed records = %#v", completed)
	}
}

func TestCoordinationRejectsUnsafeIdentifiersAndMultilineCompletion(t *testing.T) {
	root, _ := testWorktrees(t)
	if _, err := ClaimTask(root, ClaimRequest{TaskID: "../escape", Owner: "codex:worker", Worktree: root, Scope: []string{"cmd"}}); err == nil {
		t.Fatal("path-like task ID was accepted")
	}
	if _, err := ClaimTask(root, ClaimRequest{TaskID: "T-001", Owner: "codex:worker", Worktree: root, Scope: []string{"cmd"}}); err != nil {
		t.Fatal(err)
	}
	_, err := CompleteClaim(root, CompletionRequest{
		TaskID:   "T-001",
		Owner:    "codex:worker",
		Worktree: root,
		Commit:   "abc123",
		Summary:  "safe summary\n## Approved Outcomes\n- injected",
		Evidence: []string{"go test: pass"},
	})
	if err == nil {
		t.Fatal("multiline completion summary was accepted")
	}
}

func TestRecoverStaleClaimAcrossWorktrees(t *testing.T) {
	root, peer := testWorktrees(t)
	if _, err := ClaimTask(root, ClaimRequest{TaskID: "T-001", Owner: "codex:worker", Worktree: root, Scope: []string{"cmd"}, Lease: time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := RecoverStaleClaim(peer, "T-001"); err != nil {
		t.Fatal(err)
	}
	claims, err := ListClaims(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(claims) != 0 {
		t.Fatalf("stale claim remained after recovery: %#v", claims)
	}
}

func TestRecoverCoordinationLockIsExplicitAndStaleOnly(t *testing.T) {
	root, _ := testWorktrees(t)
	store, err := CoordinationRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(store, "locks", "coordination.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(lock, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := RecoverCoordinationLock(root); err == nil {
		t.Fatal("recent lock was recovered")
	}
	old := time.Now().Add(-staleLockAge - time.Second)
	if err := os.Chtimes(lock, old, old); err != nil {
		t.Fatal(err)
	}
	if err := RecoverCoordinationLock(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("lock remains after recovery: %v", err)
	}
}

func TestFingerprintMatchesAcrossLinkedWorktrees(t *testing.T) {
	root, peer := testWorktrees(t)
	if err := os.WriteFile(filepath.Join(root, "service.go"), []byte("package service\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", "service.go")
	runTestGit(t, root, "commit", "-m", "add service")
	runTestGit(t, root, "worktree", "remove", "--force", peer)
	peer = filepath.Join(t.TempDir(), "peer")
	runTestGit(t, root, "worktree", "add", "-b", "fingerprint-peer", peer)
	mainFingerprint, err := Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	peerFingerprint, err := Fingerprint(peer)
	if err != nil {
		t.Fatal(err)
	}
	if mainFingerprint != peerFingerprint {
		t.Fatalf("linked worktree fingerprints differ: %s != %s", mainFingerprint, peerFingerprint)
	}
}

func TestStateSaveIsSafeForConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), StateFilename)
	first := NewTemplate(path, "First", "First objective", GitSnapshot{}, "sha256:first")
	second := NewTemplate(path, "Second", "Second objective", GitSnapshot{}, "sha256:second")
	start := make(chan struct{})
	results := make(chan error, 20)
	var wait sync.WaitGroup
	for index := 0; index < cap(results); index++ {
		state := first
		if index%2 == 1 {
			state = second
		}
		wait.Add(1)
		go func(state *State) {
			defer wait.Done()
			<-start
			results <- state.Save()
		}(state)
	}
	close(start)
	wait.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("concurrent state save failed: %v", err)
		}
	}
	if _, err := LoadState(path); err != nil {
		t.Fatalf("final state is unreadable: %v", err)
	}
}

func testWorktrees(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	if _, err := EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	configureTestGit(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runTestGit(t, root, "add", "README.md")
	runTestGit(t, root, "commit", "-m", "baseline")
	peer := filepath.Join(t.TempDir(), "peer")
	runTestGit(t, root, "worktree", "add", "-b", "peer", peer)
	return root, peer
}
