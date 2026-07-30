package continuity

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanRoundTripAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), PlanFilename)
	plan := NewPlan(path, "Test", "Deliver the approved result.")
	if result := plan.Validate(); !result.OK() {
		t.Fatalf("new draft plan was invalid: %v", result.Errors)
	}
	milestone := Milestone{Status: ">", ID: "M-001", Priority: 1, Title: "First release", Acceptance: "Users can validate the first release."}
	outcome := Outcome{Status: " ", ID: "O-001", Milestone: milestone.ID, Priority: 1, Title: "Visible result", Scope: []string{"internal/continuity"}, Acceptance: "Focused tests pass."}
	if err := plan.ReplaceSection("Milestones", RenderMilestone(milestone)); err != nil {
		t.Fatal(err)
	}
	if err := plan.ReplaceSection("Outcomes", RenderOutcome(outcome)); err != nil {
		t.Fatal(err)
	}
	if err := plan.ReplaceSection("Sources", RenderSource(PlanSource{ID: "S-001", Role: "architecture", Authority: "inferred", Confidence: "high", Reference: "internal/continuity", Summary: "Existing portable core."})); err != nil {
		t.Fatal(err)
	}
	plan.SetMeta("status", "approved")
	plan.SetMeta("active_milestone", milestone.ID)
	plan.SetMeta("approved_at", time.Now().UTC().Format(time.RFC3339))
	plan.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
	if result := plan.Validate(); !result.OK() {
		t.Fatalf("approved plan was invalid: %v", result.Errors)
	}
	if err := plan.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := loaded.Outcomes(); err != nil || len(got) != 1 || got[0].ID != "O-001" || got[0].Scope[0] != "internal/continuity" {
		t.Fatalf("outcomes did not round trip: %#v, %v", got, err)
	}
}

func TestPlanRejectsUnknownMilestoneAndBrokenRecords(t *testing.T) {
	plan := NewPlan(filepath.Join(t.TempDir(), PlanFilename), "Test", "Test validation.")
	badOutcome := "- [ ] O-001 | milestone: M-404 | priority: 1 | title: Unknown parent | scope: internal | accepts: Must fail."
	if err := plan.ReplaceSection("Outcomes", badOutcome); err != nil {
		t.Fatal(err)
	}
	result := plan.Validate()
	if result.OK() || !strings.Contains(strings.Join(result.Errors, "; "), "unknown milestone") {
		t.Fatalf("expected unknown milestone error, got %#v", result)
	}
	if err := plan.ReplaceSection("Outcomes", "- [ ] O-001 invalid"); err != nil {
		t.Fatal(err)
	}
	result = plan.Validate()
	if result.OK() || !strings.Contains(strings.Join(result.Errors, "; "), "outcomes") {
		t.Fatalf("expected malformed outcome error, got %#v", result)
	}
}

func TestPlanDecisionAndDeltaRecords(t *testing.T) {
	plan := NewPlan(filepath.Join(t.TempDir(), PlanFilename), "Test", "Test decisions.")
	decision := DecisionCard{ID: "D-001", Impact: "high", Outcome: "none", Question: "Which store?", Recommendation: "Use SQLite.", Evidence: "Existing deployment constraints.", Resolution: "none"}
	if err := plan.ReplaceSection("Decisions", RenderDecision(decision)); err != nil {
		t.Fatal(err)
	}
	delta := PlanDelta{ID: "DLT-001", Impact: "product", Summary: "Add approved export outcome."}
	if err := plan.ReplaceSection("Deltas", RenderDelta(delta)); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Decisions(); err != nil {
		t.Fatal(err)
	}
	if _, err := plan.Deltas(); err != nil {
		t.Fatal(err)
	}
	decision.Resolved = true
	decision.Resolution = "User approved SQLite."
	updated, err := ReplacePlanRecord(plan.Section("Decisions"), decision.ID, RenderDecision(decision))
	if err != nil {
		t.Fatal(err)
	}
	if err := plan.ReplaceSection("Decisions", updated); err != nil {
		t.Fatal(err)
	}
	if got, err := plan.Decisions(); err != nil || len(got) != 1 || !got[0].Resolved {
		t.Fatalf("decision did not resolve: %#v, %v", got, err)
	}
}
