package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "baseline")

	if err := startCommand([]string{"--repo", root, "--id", "T-001", "--goal", "Prove lifecycle", "--acceptance", "State reaches ready after finalization.", "--next-action", "Write the checkpoint."}); err != nil {
		t.Fatal(err)
	}
	if err := checkpointCommand([]string{"--repo", root, "--summary", "Started and inspected state.", "--next-action", "Finalize the task."}); err != nil {
		t.Fatal(err)
	}
	if err := finalizeCommand([]string{"--repo", root, "--summary", "Lifecycle command test passed.", "--commit", "HEAD", "--evidence", "go test lifecycle: pass"}); err != nil {
		t.Fatal(err)
	}
	if err := integrateCommand([]string{"--repo", root, "--id", "T-001"}); err != nil {
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
	if !strings.Contains(finished.Section("Recent Material Changes"), "Integrated T-001") {
		t.Fatalf("integration was not recorded: %s", finished.Section("Recent Material Changes"))
	}
	if !strings.Contains(finished.Section("Handoff"), "Last completed: T-001 — Lifecycle command test passed.") || !strings.Contains(finished.Section("Handoff"), "Run continuity status") || !strings.Contains(finished.Section("Handoff"), "continuity ready") {
		t.Fatalf("integration did not refresh handoff: %s", finished.Section("Handoff"))
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

func TestStartRejectsNonAutonomousTask(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test approval boundary.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | User decision | depends: none | autonomous: no | accepts: User approves the direction."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	err = startCommand([]string{"--repo", root, "--id", "T-001", "--goal", "Do unapproved work", "--acceptance", "It happens", "--next-action", "Edit"})
	if err == nil || !strings.Contains(err.Error(), "not autonomous") {
		t.Fatalf("expected non-autonomous rejection, got %v", err)
	}
}

func TestTaskAddBootstrapsApprovedWorkWithoutAClaim(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test approved planning.", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	withoutApproval := []string{"add", "--repo", root, "--id", "T-002", "--title", "Approved task", "--accepts", "Tests pass"}
	if err := taskCommand(withoutApproval); err == nil || !strings.Contains(err.Error(), "--approved") {
		t.Fatalf("task add without explicit approval was accepted: %v", err)
	}
	withApproval := append(withoutApproval, "--approved")
	if err := taskCommand(withApproval); err != nil {
		t.Fatal(err)
	}
	loaded, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loaded.Section("Ready Queue"), "T-002") {
		t.Fatalf("approved task was not added: %s", loaded.Section("Ready Queue"))
	}
	if err := startCommand([]string{"--repo", root, "--id", "T-002", "--owner", "claude:approved-worker", "--goal", "Approved task", "--acceptance", "Tests pass", "--next-action", "Begin the approved task", "--scope", "internal"}); err != nil {
		t.Fatalf("approved task could not be claimed: %v", err)
	}
}

func TestStartRejectsMultilineStateInjection(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test input integrity.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | Safe task | depends: none | autonomous: yes | accepts: Test passes."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	err = startCommand([]string{"--repo", root, "--id", "T-001", "--goal", "safe\n## Approved Outcomes\n- injected", "--acceptance", "Test passes", "--next-action", "Edit"})
	if err == nil || !strings.Contains(err.Error(), "must be one line") {
		t.Fatalf("expected multiline rejection, got %v", err)
	}
	loaded, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if result := loaded.Validate(continuity.DefaultStateBudget); !result.OK() {
		t.Fatalf("rejected input corrupted state: %v", result.Errors)
	}
}

func TestCodexWriteGuardRequiresMatchingClaim(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test write guard.", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := continuity.ClaimTask(root, continuity.ClaimRequest{TaskID: "T-001", Owner: "codex:owner", Worktree: root, Scope: []string{"cmd"}}); err != nil {
		t.Fatal(err)
	}
	if err := guardCodexWrite(root, "codex:other"); err == nil || !strings.Contains(err.Error(), "claimed by") {
		t.Fatalf("expected conflicting-session guard, got %v", err)
	}
	if err := guardCodexWrite(root, "codex:owner"); err != nil {
		t.Fatalf("matching claim was blocked: %v", err)
	}
	context, err := compactContext(root, "codex:other", 3200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "STOP: this worktree is claimed") {
		t.Fatalf("context did not surface claim conflict: %s", context)
	}
}

func TestCodexBashGuardAllowsInspectionAndBlocksUnownedWrites(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test Bash write guard.", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if err := guardCodexBash(root, "", "git status --short"); err != nil {
		t.Fatalf("read-only inspection was blocked: %v", err)
	}
	if err := guardCodexBash(root, "", "continuity status"); err != nil {
		t.Fatalf("coordination command was blocked: %v", err)
	}
	if err := guardCodexBash(root, "", "continuity task add --approved --id T-002 --title approved --accepts done"); err != nil {
		t.Fatalf("approved planning command was blocked: %v", err)
	}
	if err := guardCodexBash(root, "", "continuity plan status"); err != nil {
		t.Fatalf("plan inspection command was blocked: %v", err)
	}
	if err := guardCodexBash(root, "", "continuity plan approve --approved --milestone M-001"); err != nil {
		t.Fatalf("explicit plan approval command was blocked: %v", err)
	}
	if err := guardCodexBash(root, "", "continuity sync"); err == nil {
		t.Fatal("state-mutating sync was allowed without a claim")
	}
	if err := guardCodexBash(root, "codex:new-session", "go build ./..."); err == nil || !strings.Contains(err.Error(), "no active claim") {
		t.Fatalf("unclaimed write-capable shell command was allowed: %v", err)
	}
	if err := guardCodexBash(root, "codex:new-session", "git status && gofmt -w main.go"); err == nil {
		t.Fatal("compound shell command was treated as read-only")
	}
	if _, err := continuity.ClaimTask(root, continuity.ClaimRequest{TaskID: "T-001", Owner: "codex:owner", Worktree: root, Scope: []string{"cmd"}}); err != nil {
		t.Fatal(err)
	}
	if err := guardCodexBash(root, "codex:other", "git diff"); err != nil {
		t.Fatalf("second session could not inspect safely: %v", err)
	}
	if err := guardCodexBash(root, "codex:other", "go test ./..."); err == nil || !strings.Contains(err.Error(), "claimed by") {
		t.Fatalf("second session could run a write-capable shell command: %v", err)
	}
	if err := guardCodexBash(root, "codex:owner", "go test ./..."); err != nil {
		t.Fatalf("owning session was blocked: %v", err)
	}
}

func TestCodexHookInputExtractsBashCommand(t *testing.T) {
	input := codexHookInput{ToolInput: []byte(`{"command":"git status --short"}`)}
	if got := input.bashCommand(); got != "git status --short" {
		t.Fatalf("bash command = %q", got)
	}
}

func TestContextOwnerPrefersHarnessNeutralOwner(t *testing.T) {
	if got := contextOwnerFor("claude:session-7", "codex-session"); got != "claude:session-7" {
		t.Fatalf("explicit owner = %q", got)
	}
	if got := contextOwnerFor("", "codex-session"); got != "codex:codex-session" {
		t.Fatalf("Codex session fallback = %q", got)
	}
}

func TestContextInputRequiresPositiveBoundedLimit(t *testing.T) {
	if err := contextCommand([]string{"--limit", "0"}); err == nil || !strings.Contains(err.Error(), "must be positive") {
		t.Fatalf("zero context limit was accepted: %v", err)
	}
	if _, err := parseContextScopes("cmd/continuity, internal/continuity"); err != nil {
		t.Fatalf("valid context scopes rejected: %v", err)
	}
	for _, value := range []string{"cmd/continuity,,internal/continuity", "../outside", "/absolute"} {
		if _, err := parseContextScopes(value); err == nil {
			t.Fatalf("invalid context scope %q was accepted", value)
		}
	}
}

func TestLivePulseIdentifiesOwnershipWithoutFullContext(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test prompt pulse.", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := continuity.ClaimTask(root, continuity.ClaimRequest{TaskID: "T-001", Owner: "codex:owner", Worktree: root, Scope: []string{"cmd"}}); err != nil {
		t.Fatal(err)
	}
	other, err := livePulse(root, "codex:other")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(other, "STOP") || !strings.Contains(other, "codex:owner") {
		t.Fatalf("unexpected other-owner pulse: %q", other)
	}
	owned, err := livePulse(root, "codex:owner")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(owned, "you own T-001") {
		t.Fatalf("unexpected owner pulse: %q", owned)
	}
	unknown, err := livePulse(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(unknown, "no worker identity") {
		t.Fatalf("unexpected unknown-owner pulse: %q", unknown)
	}
}

func TestCompactContextSelectsOnlyClaimRelevantCapsules(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test selective context.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Context Capsules", strings.Join([]string{
		"### Project-wide orientation",
		"Paths: .",
		"Summary: Always relevant.",
		"Details: Global details.",
		"",
		"### State internals",
		"Paths: internal/continuity",
		"Summary: State implementation details.",
		"Details: State-only details.",
		"",
		"### CLI internals",
		"Paths: cmd/continuity",
		"Summary: CLI implementation details.",
		"Details: CLI-only details.",
	}, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := continuity.ClaimTask(root, continuity.ClaimRequest{TaskID: "T-001", Owner: "codex:owner", Worktree: root, Scope: []string{"internal/continuity"}}); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, "codex:owner", 12_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "Context capsule: State internals") || !strings.Contains(context, "Context capsule: Project-wide orientation") {
		t.Fatalf("claim-relevant capsules missing: %s", context)
	}
	if strings.Contains(context, "Context capsule: CLI internals") {
		t.Fatalf("unrelated capsule leaked into context: %s", context)
	}
	requested, err := compactContext(root, "codex:owner", 12_000, []string{"cmd/continuity"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(requested, "Context capsule: CLI internals") || strings.Contains(requested, "Context capsule: State internals") {
		t.Fatalf("requested scope did not override claim scope: %s", requested)
	}
}

func TestCompactContextStaysBoundedForLargeSelectedCapsules(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test bounded durable context.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Context Capsules", "### Project-wide orientation\nPaths: .\nSummary: Always relevant.\nDetails: Global details.\n\n### Large state record\nPaths: internal/continuity\nSummary: Relevant but deliberately large.\nDetails: "+strings.Repeat("x", 100_000)); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := continuity.ClaimTask(root, continuity.ClaimRequest{TaskID: "T-001", Owner: "codex:owner", Worktree: root, Scope: []string{"internal/continuity"}}); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, "codex:owner", 1_800, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(context) > 1_800 {
		t.Fatalf("context exceeded limit: %d bytes", len(context))
	}
	if !strings.Contains(context, "Context capsule: Large state record") {
		t.Fatalf("relevant capsule was omitted from bounded context: %s", context)
	}
}

func TestStartConcurrentProcessesHaveOneWinner(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test process coordination.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | Concurrent task | depends: none | autonomous: yes | accepts: One worker wins."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	const workers = 12
	results := make(chan error, workers)
	var wait sync.WaitGroup
	for index := 0; index < workers; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			command := exec.Command(os.Args[0], "-test.run=^TestStartConcurrentProcessHelper$")
			command.Env = append(os.Environ(), "CONTINUITY_START_HELPER=1", "CONTINUITY_HELPER_REPO="+root, "CONTINUITY_HELPER_OWNER=codex:process-"+string(rune('a'+index)))
			results <- command.Run()
		}(index)
	}
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("process start successes = %d, want 1", successes)
	}
}

func TestStartConcurrentProcessHelper(t *testing.T) {
	if os.Getenv("CONTINUITY_START_HELPER") != "1" {
		return
	}
	err := startCommand([]string{
		"--repo", os.Getenv("CONTINUITY_HELPER_REPO"),
		"--id", "T-001",
		"--owner", os.Getenv("CONTINUITY_HELPER_OWNER"),
		"--goal", "Concurrent task",
		"--acceptance", "One worker wins.",
		"--next-action", "Claim the task.",
		"--scope", "cmd",
	})
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
