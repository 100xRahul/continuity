package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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
	if err := state.ReplaceSection("Ready Queue", strings.Join([]string{
		"- [ ] T-001 | Prove lifecycle | depends: none | autonomous: yes | accepts: State reaches ready after finalization.",
		"- [ ] T-002 | Resume after lifecycle | depends: T-001 | autonomous: yes | accepts: The follow-up task is ready.",
	}, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "baseline")

	if err := startCommand([]string{"--repo", root, "--id", "T-001", "--owner", "claude:lifecycle", "--goal", "Prove lifecycle", "--acceptance", "State reaches ready after finalization.", "--next-action", "Write the checkpoint."}); err != nil {
		t.Fatal(err)
	}
	if err := checkpointCommand([]string{"--repo", root, "--owner", "claude:lifecycle", "--summary", "Started and inspected state.", "--next-action", "Finalize the task."}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "result.go"), []byte("package result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", "result.go")
	runGitCommand(t, root, "commit", "-m", "worker result")
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
	if !strings.Contains(finished.Section("Handoff"), "Last completed: T-001 — Lifecycle command test passed.") || !strings.Contains(finished.Section("Handoff"), "continuity context") || !strings.Contains(finished.Section("Handoff"), "claim T-002") {
		t.Fatalf("integration did not refresh handoff: %s", finished.Section("Handoff"))
	}
	archive, err := os.ReadFile(filepath.Join(root, continuity.PlanHistoryDir, "changes", "T-001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(archive), "- Intent: Prove lifecycle") || !strings.Contains(string(archive), "`result.go`") || !strings.Contains(string(archive), "go test lifecycle: pass") || !strings.Contains(string(archive), "T-002") {
		t.Fatalf("change archive is incomplete: %s", archive)
	}
	capsules, err := finished.ContextCapsules()
	if err != nil {
		t.Fatal(err)
	}
	foundCapsule := false
	for _, capsule := range capsules {
		if capsule.Name != "Change T-001" {
			continue
		}
		foundCapsule = strings.Contains(strings.Join(capsule.Paths, ","), "result.go") && strings.Contains(capsule.Details, "Next: T-002")
	}
	if !foundCapsule {
		t.Fatalf("integration did not create a scoped change capsule: %#v", capsules)
	}
	context, err := compactContext(root, "", 12_000, []string{"result.go"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "Context capsule: Change T-001") {
		t.Fatalf("scoped context omitted the change capsule: %s", context)
	}
	if _, err := os.Stat(filepath.Join(root, continuity.StateFilename)); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationAgesOverlappingFactBeforeSuggestingNextTask(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test integration fact aging.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", strings.Join([]string{
		"- [ ] T-001 | Change the result | depends: none | autonomous: yes | accepts: Result is written.",
		"- [ ] T-002 | Review the result | depends: T-001 | autonomous: yes | accepts: Follow-up is ready.",
	}, "\n")); err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceFactCards([]continuity.FactCard{{
		Status:     "fresh",
		ID:         "F-001",
		Source:     "D-001",
		Paths:      []string{"result.go"},
		VerifiedAt: time.Now().UTC(),
		Evidence:   "Initial result review passed.",
		Summary:    "Result behavior needs review after source changes.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	runGitCommand(t, root, "add", ".")
	runGitCommand(t, root, "commit", "-m", "baseline")

	if err := startCommand([]string{"--repo", root, "--owner", "test:worker", "--id", "T-001", "--goal", "Change the result", "--acceptance", "Result is written.", "--next-action", "Write result.go.", "--scope", "result.go"}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "result.go"), []byte("package result\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "add", "result.go")
	runGitCommand(t, root, "commit", "-m", "worker result")
	if err := finalizeCommand([]string{"--repo", root, "--owner", "test:worker", "--id", "T-001", "--summary", "Result changed.", "--commit", "HEAD", "--evidence", "focused test: pass"}); err != nil {
		t.Fatal(err)
	}
	if err := integrateCommand([]string{"--repo", root, "--id", "T-001"}); err != nil {
		t.Fatal(err)
	}

	finished, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	facts, err := finished.FactCards()
	if err != nil || len(facts) != 1 || facts[0].Status != "stale" {
		t.Fatalf("integration did not age overlapping fact: %#v, %v", facts, err)
	}
	if handoff := finished.Section("Handoff"); !strings.Contains(handoff, "fact confirm --id F-001") {
		t.Fatalf("handoff suggested work without confirmation: %s", handoff)
	}
	err = startCommand([]string{"--repo", root, "--owner", "test:worker", "--id", "T-002", "--goal", "Review the result", "--acceptance", "Follow-up is ready.", "--next-action", "Review result.go."})
	if err == nil || !strings.Contains(err.Error(), "stale fact") {
		t.Fatalf("stale fact did not prevent follow-up task: %v", err)
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

func TestTaskAddRejectsArchivedTaskIDReuse(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	if err := writeHistoryFile(root, changeArchivePath("T-002"), "# Change T-002\n"); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Reject archived IDs.", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	err = taskCommand([]string{"add", "--repo", root, "--id", "T-002", "--title", "Reuse old ID", "--accepts", "Must fail", "--approved"})
	if err == nil || !strings.Contains(err.Error(), "committed change archive") {
		t.Fatalf("archived task ID was reusable: %v", err)
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
	input := hookInput{ToolInput: []byte(`{"command":"git status --short"}`)}
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

func TestCompactContextNamesOneSafeRestartAction(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test restart guidance.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | Resume approved work | depends: none | autonomous: yes | accepts: The work resumes safely."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, "claude:restart", 12_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "Next approved task: T-001") || !strings.Contains(context, "Immediate next action: continuity start --id T-001") {
		t.Fatalf("context did not provide one actionable restart path: %s", context)
	}
	if !strings.Contains(context, "highest-ranked unclaimed autonomous task") {
		t.Fatalf("context omitted selection evidence: %s", context)
	}
}

func TestCompactContextReportsGateInsteadOfRecommendingBlockedTask(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test gated restart guidance.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | Resume approved work | depends: none | autonomous: yes | accepts: The work resumes safely."); err != nil {
		t.Fatal(err)
	}
	if err := state.ReplaceFactCards([]continuity.FactCard{{
		Status:     "stale",
		ID:         "F-001",
		Source:     "D-001",
		Paths:      []string{"service"},
		VerifiedAt: time.Now().UTC(),
		Evidence:   "Earlier service review.",
		Summary:    "Service assumptions need review.",
	}}); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, "codex:gated", 3_200, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "STOP: approved work is gated: F-001") || !strings.Contains(context, "continuity fact confirm --id F-001") {
		t.Fatalf("context omitted the exact stale-fact recovery action: %s", context)
	}
	if strings.Contains(context, "Next approved task:") || strings.Contains(context, "continuity start --id T-001") {
		t.Fatalf("context recommended blocked work: %s", context)
	}
}

func TestAssembleContextPacketPreservesWholeSafetyBlock(t *testing.T) {
	critical := "owner: codex:test\nSTOP: resolve F-001\nImmediate next action: confirm F-001\n"
	sections := []contextPacketSection{{full: strings.Repeat("optional detail ", 200)}}
	packet, err := assembleContextPacket(220, critical, sections)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(packet, critical) || len(packet) > 220 {
		t.Fatalf("safety block was altered or packet exceeded its limit: %q", packet)
	}
	if strings.Contains(packet, "optional det") {
		t.Fatalf("optional section was byte-truncated into the packet: %q", packet)
	}
	if _, err := assembleContextPacket(len(critical)-1, critical, nil); err == nil || !strings.Contains(err.Error(), "too small") {
		t.Fatalf("unsafe limit did not fail explicitly: %v", err)
	}
}

func TestShellQuoteMakesRestartIdentityLiteral(t *testing.T) {
	quoted := shellQuote("harness:$(touch nope)'worker")
	if quoted != `'harness:$(touch nope)'"'"'worker'` {
		t.Fatalf("unsafe shell quote: %s", quoted)
	}
}

func TestCompactContextDoesNotRecommendWorkFromStaleState(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test stale restart guidance.", continuity.Snapshot(root), fingerprint)
	if err := state.ReplaceSection("Ready Queue", "- [ ] T-001 | Unsafe stale work | depends: none | autonomous: yes | accepts: Stale work is not suggested."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "changed.go"), []byte("package changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, "codex:stale", 12_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "STOP: durable state is not safe to claim") {
		t.Fatalf("context omitted stale-state gate: %s", context)
	}
	if strings.Contains(context, "Next approved task:") || strings.Contains(context, "continuity start --id T-001") {
		t.Fatalf("context recommended work from stale state: %s", context)
	}
}

func TestOwnedStaleClaimRenewsWithoutUserRecovery(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	if _, err := continuity.ClaimTask(root, continuity.ClaimRequest{TaskID: "T-001", Owner: "codex:returning", Worktree: root, Scope: []string{"cmd"}, Lease: time.Nanosecond}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Millisecond)
	claim, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		t.Fatal(err)
	}
	if claim == nil || !claim.Stale(time.Now().UTC()) {
		t.Fatalf("test claim did not become stale: %#v", claim)
	}
	heartbeatOwnedClaim(root, "codex:returning")
	claim, err = continuity.FindClaimForWorktree(root)
	if err != nil {
		t.Fatal(err)
	}
	if claim == nil || claim.Stale(time.Now().UTC()) {
		t.Fatalf("returning owner did not renew its unrecovered claim: %#v", claim)
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
