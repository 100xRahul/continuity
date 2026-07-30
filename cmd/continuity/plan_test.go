package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"continuity/internal/continuity"
)

func TestPlanDraftApprovalActivationAndDelta(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--project", "Test", "--objective", "Deliver a tested plan."}); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	if err := planMilestoneCommand([]string{"add", "--repo", root, "--id", "M-001", "--priority", "1", "--title", "First milestone", "--accepts", "First milestone works."}); err != nil {
		t.Fatal(err)
	}
	if err := planMilestoneCommand([]string{"add", "--repo", root, "--id", "M-002", "--priority", "2", "--title", "Second milestone", "--accepts", "Second milestone works."}); err != nil {
		t.Fatal(err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-001", "--milestone", "M-001", "--priority", "1", "--title", "First result", "--scope", "internal/continuity", "--accepts", "Focused tests pass."}); err != nil {
		t.Fatal(err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-002", "--milestone", "M-002", "--priority", "1", "--title", "Second result", "--scope", "cmd/continuity", "--accepts", "Focused tests pass."}); err != nil {
		t.Fatal(err)
	}
	if err := planDecisionAddCommand([]string{"--repo", root, "--id", "D-001", "--impact", "high", "--outcome", "O-001", "--question", "Which store?", "--recommendation", "Use SQLite.", "--evidence", "Local deployment."}); err != nil {
		t.Fatal(err)
	}
	if err := planApproveCommand([]string{"--repo", root, "--approved", "--milestone", "M-001"}); err == nil || !strings.Contains(err.Error(), "high-impact decision") {
		t.Fatalf("approval ignored unresolved high-impact decision: %v", err)
	}
	if err := planDecisionResolveCommand([]string{"--repo", root, "--approved", "--id", "D-001", "--resolution", "User approved SQLite."}); err != nil {
		t.Fatal(err)
	}
	if err := planApproveCommand([]string{"--repo", root, "--approved", "--milestone", "M-001"}); err != nil {
		t.Fatal(err)
	}
	plan, err := continuity.LoadPlan(filepath.Join(root, continuity.PlanFilename))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Meta["status"] != "approved" || plan.Meta["active_milestone"] != "M-001" {
		t.Fatalf("unexpected approved plan metadata: %#v", plan.Meta)
	}
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	if state.Meta["plan_status"] != "approved" || !strings.Contains(state.Section("Approved Outcomes"), "O-001") {
		t.Fatalf("state did not project active plan: %#v %q", state.Meta, state.Section("Approved Outcomes"))
	}

	if err := state.ReplaceSection("Ready Queue", "- [x] T-001 | Complete first outcome | outcome: O-001 | scope: internal/continuity | depends: none | autonomous: yes | validates: go test ./internal/continuity | accepts: Focused tests pass. | derived: It implements the first approved result."); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	next, err := selectNextWork(root, state)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(next.Summary, "O-001") || !strings.Contains(next.Action, "plan outcome complete --id O-001") {
		t.Fatalf("completed outcome did not become the next handoff: %#v", next)
	}
	if err := planOutcomeCommand([]string{"complete", "--repo", root, "--id", "O-001", "--evidence", "go test ./internal/continuity: pass"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, continuity.PlanHistoryDir, "outcomes", "O-001.md")); err != nil {
		t.Fatal(err)
	}
	if err := planActivateCommand([]string{"--repo", root, "--approved", "--milestone", "M-002"}); err != nil {
		t.Fatal(err)
	}
	if err := planDeltaAddCommand([]string{"--repo", root, "--id", "DLT-001", "--impact", "product", "--summary", "Add approved export work."}); err != nil {
		t.Fatal(err)
	}
	if err := planDeltaApproveCommand([]string{"--repo", root, "--approved", "--id", "DLT-001"}); err != nil {
		t.Fatal(err)
	}
	plan, err = continuity.LoadPlan(filepath.Join(root, continuity.PlanFilename))
	if err != nil {
		t.Fatal(err)
	}
	if plan.Meta["active_milestone"] != "M-002" {
		t.Fatalf("next milestone was not activated: %#v", plan.Meta)
	}
	outcomes, err := plan.Outcomes()
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) < 1 || outcomes[0].Status != "x" {
		t.Fatalf("completed outcome was not recorded: %#v", outcomes)
	}
	milestoneArchive, err := os.ReadFile(filepath.Join(root, continuity.PlanHistoryDir, "milestones", "M-001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(milestoneArchive), "- [x] M-001") || !strings.Contains(string(milestoneArchive), "O-001") {
		t.Fatalf("milestone archive omitted completed snapshot: %s", milestoneArchive)
	}
	deltas, err := plan.Deltas()
	if err != nil || len(deltas) != 1 || !deltas[0].Approved {
		t.Fatalf("approved delta missing: %#v, %v", deltas, err)
	}
}

func TestPlanCommandsRequireApprovalAndDraftBoundaries(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--objective", "Test plan command boundaries."}); err != nil {
		t.Fatal(err)
	}
	if err := planMilestoneCommand([]string{"add", "--repo", root, "--id", "M-001", "--priority", "1", "--title", "First", "--accepts", "Works."}); err != nil {
		t.Fatal(err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-001", "--milestone", "M-001", "--priority", "1", "--title", "Result", "--scope", "internal", "--accepts", "Works."}); err != nil {
		t.Fatal(err)
	}
	if err := planApproveCommand([]string{"--repo", root, "--milestone", "M-001"}); err == nil || !strings.Contains(err.Error(), "--approved") {
		t.Fatalf("plan approval without explicit acknowledgement succeeded: %v", err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-404", "--milestone", "M-404", "--priority", "1", "--title", "Bad", "--scope", "internal", "--accepts", "Fails."}); err == nil || !strings.Contains(err.Error(), "unknown milestone") {
		t.Fatalf("unknown milestone outcome was accepted: %v", err)
	}
}

func TestPlanOutcomeCompletionRejectsUnintegratedTask(t *testing.T) {
	root := t.TempDir()
	if err := initCommand([]string{"--repo", root, "--objective", "Test outcome completion boundary."}); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, root, "config", "user.name", "Continuity Test")
	runGitCommand(t, root, "config", "user.email", "continuity@example.test")
	if err := planMilestoneCommand([]string{"add", "--repo", root, "--id", "M-001", "--priority", "1", "--title", "First", "--accepts", "Works."}); err != nil {
		t.Fatal(err)
	}
	if err := planOutcomeCommand([]string{"add", "--repo", root, "--id", "O-001", "--milestone", "M-001", "--priority", "1", "--title", "Result", "--scope", "internal/continuity", "--accepts", "Focused tests pass."}); err != nil {
		t.Fatal(err)
	}
	if err := planApproveCommand([]string{"--repo", root, "--approved", "--milestone", "M-001"}); err != nil {
		t.Fatal(err)
	}
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if err != nil {
		t.Fatal(err)
	}
	queue := "- [ ] T-001 | Incomplete result | outcome: O-001 | scope: internal/continuity | depends: none | autonomous: yes | validates: go test ./internal/continuity | accepts: Focused tests pass. | derived: It implements the approved result."
	if err := state.ReplaceSection("Ready Queue", queue); err != nil {
		t.Fatal(err)
	}
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	err = planOutcomeCommand([]string{"complete", "--repo", root, "--id", "O-001", "--evidence", "go test ./internal/continuity: pass"})
	if err == nil || !strings.Contains(err.Error(), "unfinished task") {
		t.Fatalf("unfinished outcome task was accepted: %v", err)
	}
}

func TestCompactContextProjectsOnlyRelevantActivePlanOutcome(t *testing.T) {
	root := t.TempDir()
	if _, err := continuity.EnsureGitRepo(root); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		t.Fatal(err)
	}
	state := continuity.NewTemplate(filepath.Join(root, continuity.StateFilename), "Test", "Test plan projection.", continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		t.Fatal(err)
	}
	plan := continuity.NewPlan(filepath.Join(root, continuity.PlanFilename), "Test", "Deliver approved outcomes.")
	if err := plan.ReplaceSection("Milestones", continuity.RenderMilestone(continuity.Milestone{Status: ">", ID: "M-001", Priority: 1, Title: "Current", Acceptance: "Current milestone passes."})); err != nil {
		t.Fatal(err)
	}
	outcomes := []string{
		continuity.RenderOutcome(continuity.Outcome{Status: " ", ID: "O-001", Milestone: "M-001", Priority: 1, Title: "State result", Scope: []string{"internal/continuity"}, Acceptance: "State test passes."}),
		continuity.RenderOutcome(continuity.Outcome{Status: " ", ID: "O-002", Milestone: "M-001", Priority: 2, Title: "CLI result", Scope: []string{"cmd/continuity"}, Acceptance: "CLI test passes."}),
	}
	if err := plan.ReplaceSection("Outcomes", strings.Join(outcomes, "\n")); err != nil {
		t.Fatal(err)
	}
	plan.SetMeta("status", "approved")
	plan.SetMeta("active_milestone", "M-001")
	plan.SetMeta("approved_at", "2026-01-01T00:00:00Z")
	plan.SetMeta("updated_at", "2026-01-01T00:00:00Z")
	if err := plan.Save(); err != nil {
		t.Fatal(err)
	}
	context, err := compactContext(root, "", 12_000, []string{"internal/continuity"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(context, "Active approved outcome: O-001") || strings.Contains(context, "Active approved outcome: O-002") {
		t.Fatalf("plan projection was not scope-selective: %s", context)
	}
}
