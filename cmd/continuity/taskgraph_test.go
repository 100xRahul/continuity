package main

import (
	"path/filepath"
	"strings"
	"testing"

	"continuity/internal/continuity"
)

func TestApprovedPlanAllowsOnlyBoundedOutcomeTasks(t *testing.T) {
	root := approvedTaskGraphPlan(t)
	base := []string{
		"add", "--repo", root,
		"--id", "T-101",
		"--title", "Add planned task contract",
		"--accepts", "Task contract is persisted.",
		"--outcome", "O-001",
		"--scope", "internal",
		"--validates", "go test ./internal/continuity",
		"--derived",
		"--rationale", "It establishes the focused task boundary before other implementation work.",
	}
	if err := taskCommand(base); err == nil || !strings.Contains(err.Error(), "exceeds outcome") {
		t.Fatalf("out-of-outcome scope was accepted: %v", err)
	}
	withoutProvenance := append([]string{}, base...)
	withoutProvenance = removeTaskGraphFlag(withoutProvenance, "--derived")
	withoutProvenance = removeTaskGraphFlagValue(withoutProvenance, "--rationale")
	withoutProvenance = replaceTaskGraphFlagValue(withoutProvenance, "--scope", "internal/continuity")
	if err := taskCommand(withoutProvenance); err == nil || !strings.Contains(err.Error(), "exactly one provenance") {
		t.Fatalf("unattributed planned task was accepted: %v", err)
	}
	valid := replaceTaskGraphFlagValue(base, "--scope", "internal/continuity")
	if err := taskCommand(valid); err != nil {
		t.Fatal(err)
	}
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		t.Fatal(err)
	}
	var found continuity.Task
	for _, task := range tasks {
		if task.ID == "T-101" {
			found = task
		}
	}
	if found.Outcome != "O-001" || found.Validation != "go test ./internal/continuity" || found.Rationale == "" {
		t.Fatalf("derived task contract not persisted: %#v", found)
	}
	if message := runGitCommand(t, root, "log", "-1", "--format=%s", "--", continuity.StateFilename); !strings.Contains(message, "docs(tasks): queue T-101") {
		t.Fatalf("planned task queue was not committed: %q", message)
	}
	if err := checkCommand([]string{"--repo", root}); err != nil {
		t.Fatalf("approved task graph failed validation: %v", err)
	}
}

func TestPlannedStartUsesContractDefaultsAndRecordsSelection(t *testing.T) {
	root := approvedTaskGraphPlan(t)
	addApprovedTaskGraphTask(t, root, "T-101", "O-001", "internal/continuity", "none")

	withoutReason := []string{"--repo", root, "--id", "T-101", "--owner", "claude:graph-worker"}
	if err := startCommand(withoutReason); err == nil || !strings.Contains(err.Error(), "--selection-reason") {
		t.Fatalf("planned task started without a selection reason: %v", err)
	}
	wrongContract := append(append([]string{}, withoutReason...), "--goal", "Broaden the project", "--selection-reason", "It is the top-priority ready outcome.")
	if err := startCommand(wrongContract); err == nil || !strings.Contains(err.Error(), "goal is fixed") {
		t.Fatalf("planned task accepted a replacement goal: %v", err)
	}
	tooBroad := append(append([]string{}, withoutReason...), "--selection-reason", "It is the top-priority ready outcome.", "--scope", "internal")
	if err := startCommand(tooBroad); err == nil || !strings.Contains(err.Error(), "exceeds task") {
		t.Fatalf("planned task accepted a broad claim scope: %v", err)
	}
	args := append(append([]string{}, withoutReason...), "--selection-reason", "It is the highest-priority ready outcome and unblocks its focused implementation.")
	if err := startCommand(args); err != nil {
		t.Fatal(err)
	}
	claim, err := continuity.ClaimForTask(root, "T-101")
	if err != nil {
		t.Fatal(err)
	}
	if claim == nil || claim.Goal != "Add T-101" || claim.Acceptance != "T-101 is verified." || claim.Validation != "go test ./internal/continuity" || strings.Join(claim.Scope, ",") != "internal/continuity" {
		t.Fatalf("planned claim did not inherit the contract: %#v", claim)
	}
	if !strings.Contains(claim.SelectionReason, "highest-priority") {
		t.Fatalf("selection reason was not persisted: %#v", claim)
	}
	context, err := compactContext(root, "claude:graph-worker", 12_000, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "Selection reason:") {
		t.Fatalf("bounded context omitted selection rationale: %s", context)
	}
	if err := continuity.ReleaseClaim(root, "T-101", "claude:graph-worker", root); err != nil {
		t.Fatal(err)
	}
}

func TestDraftPlanBlocksImplementationClaims(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--objective", "Do not execute before approval."}); err != nil {
		t.Fatal(err)
	}
	err := startCommand([]string{"--repo", root, "--id", "T-001", "--owner", "cursor:draft-worker"})
	if err == nil || !strings.Contains(err.Error(), "still a draft") {
		t.Fatalf("draft plan allowed an implementation claim: %v", err)
	}
}

func TestCheckRejectsAutonomousTaskOutsideActiveOutcome(t *testing.T) {
	root := approvedTaskGraphPlan(t)
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	queue := state.Section("Ready Queue") + "\n- [ ] T-101 | Escape outcome scope | outcome: O-001 | scope: internal | depends: none | autonomous: yes | validates: go test ./internal | accepts: Must not be accepted."
	if err := state.ReplaceSection("Ready Queue", queue); err != nil {
		t.Fatal(err)
	}
	if err := synchronize(state, root); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	err = checkCommand([]string{"--repo", root})
	if err == nil || !strings.Contains(err.Error(), "state check failed") {
		t.Fatalf("check accepted task outside active outcome: %v", err)
	}
}

func TestReadyCandidatesRankOutcomePriorityThenUnblocking(t *testing.T) {
	root := approvedTaskGraphPlan(t)
	addApprovedTaskGraphTask(t, root, "T-101", "O-001", "internal/continuity", "none")
	addApprovedTaskGraphTask(t, root, "T-102", "O-001", "internal/continuity", "T-101")
	addApprovedTaskGraphTask(t, root, "T-201", "O-002", "cmd/continuity", "none")
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	plan, err := continuity.LoadPlan(filepath.Join(root, continuity.PlanFilename))
	if err != nil {
		t.Fatal(err)
	}
	candidates, err := readyCandidates(state, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 2 || candidates[0].Task.ID != "T-101" || candidates[0].Unblocks != 1 || candidates[1].Task.ID != "T-201" {
		t.Fatalf("unexpected ready candidate order: %#v", candidates)
	}
	if blocked := blockedAutonomousTasks(state, nil); len(blocked) != 1 || blocked[0] != "T-102 waits for T-101" {
		t.Fatalf("blocked task evidence = %#v", blocked)
	}
}

func approvedTaskGraphPlan(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--project", "Task graph", "--objective", "Test bounded autonomous work."}); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	if err := planMilestoneCommand([]string{"add", "--repo", root, "--id", "M-001", "--priority", "1", "--title", "Task graph", "--accepts", "Bounded task graph works."}); err != nil {
		t.Fatal(err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-001", "--milestone", "M-001", "--priority", "1", "--title", "Internal task graph", "--scope", "internal/continuity", "--accepts", "Internal task graph tests pass."}); err != nil {
		t.Fatal(err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-002", "--milestone", "M-001", "--priority", "2", "--title", "CLI task graph", "--scope", "cmd/continuity", "--accepts", "CLI task graph tests pass."}); err != nil {
		t.Fatal(err)
	}
	if err := planApproveCommand([]string{"--repo", root, "--approved", "--milestone", "M-001"}); err != nil {
		t.Fatal(err)
	}
	return root
}

func addApprovedTaskGraphTask(t *testing.T, root, id, outcome, scope, depends string) {
	t.Helper()
	if err := taskCommand([]string{
		"add", "--repo", root,
		"--id", id,
		"--title", "Add " + id,
		"--accepts", id + " is verified.",
		"--outcome", outcome,
		"--scope", scope,
		"--depends", depends,
		"--validates", "go test ./" + scope,
		"--derived",
		"--rationale", "It is a focused technical step inside the approved outcome.",
	}); err != nil {
		t.Fatal(err)
	}
}

func removeTaskGraphFlag(args []string, flag string) []string {
	filtered := make([]string, 0, len(args))
	for _, value := range args {
		if value != flag {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

func removeTaskGraphFlagValue(args []string, flag string) []string {
	filtered := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		if args[index] == flag {
			index++
			continue
		}
		filtered = append(filtered, args[index])
	}
	return filtered
}

func replaceTaskGraphFlagValue(args []string, flag, replacement string) []string {
	updated := append([]string(nil), args...)
	for index := 0; index+1 < len(updated); index++ {
		if updated[index] == flag {
			updated[index+1] = replacement
			return updated
		}
	}
	return updated
}
