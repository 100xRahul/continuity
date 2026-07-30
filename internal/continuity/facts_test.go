package continuity

import (
	"strings"
	"testing"
	"time"
)

func TestFactCardsAgeOnlyOverlappingFreshEvidence(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Test", "Test source-linked facts.", GitSnapshot{}, "sha256:test")
	verified := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	cards := []FactCard{
		{Status: "fresh", ID: "F-001", Source: "D-001", Paths: []string{"internal/continuity"}, VerifiedAt: verified, Evidence: "Parser review passed.", Summary: "State parser boundary remains valid."},
		{Status: "fresh", ID: "F-002", Source: "D-002", Paths: []string{"cmd/continuity"}, VerifiedAt: verified, Evidence: "CLI review passed.", Summary: "CLI boundary remains valid."},
		{Status: "stale", ID: "F-003", Source: "D-003", Paths: []string{"README.md"}, VerifiedAt: verified, Evidence: "Docs need review.", Summary: "Documentation fact is already stale."},
	}
	if err := state.ReplaceFactCards(cards); err != nil {
		t.Fatal(err)
	}
	aged, err := state.AgeFactCards([]string{"internal/continuity/state.go"})
	if err != nil {
		t.Fatal(err)
	}
	if len(aged) != 1 || aged[0].ID != "F-001" || aged[0].Status != "stale" {
		t.Fatalf("aged facts = %#v", aged)
	}
	parsed, err := state.FactCards()
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[string]string{}
	for _, card := range parsed {
		statuses[card.ID] = card.Status
	}
	if got, want := statuses["F-001"], "stale"; got != want {
		t.Fatalf("F-001 status = %q, want %q", got, want)
	}
	if got, want := statuses["F-002"], "fresh"; got != want {
		t.Fatalf("F-002 status = %q, want %q", got, want)
	}
	if got, want := statuses["F-003"], "stale"; got != want {
		t.Fatalf("F-003 status = %q, want %q", got, want)
	}
	if result := state.Validate(DefaultStateBudget); !result.OK() {
		t.Fatalf("fact cards made state invalid: %v", result.Errors)
	}
}

func TestBlockerCardsRoundTripAndRejectUnsafeResolution(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Test", "Test blockers.", GitSnapshot{}, "sha256:test")
	blocker := BlockerCard{Status: "open", ID: "B-001", TaskID: "T-001", Scope: []string{"internal/continuity"}, Hypothesis: "Parser input has an unhandled legacy shape.", Evidence: "Focused parser test fails.", NextAction: "Create a minimal failing fixture.", Resolution: "none"}
	if err := state.ReplaceBlockerCards([]BlockerCard{blocker}); err != nil {
		t.Fatal(err)
	}
	parsed, err := state.BlockerCards()
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 1 || parsed[0].ID != blocker.ID || parsed[0].Status != "open" {
		t.Fatalf("blocker cards = %#v", parsed)
	}
	_, err = ParseBlockerCards("- [open] B-002 | task: T-001 | scope: internal | hypothesis: no | evidence: no | next: no | resolution: resolved")
	if err == nil || !strings.Contains(err.Error(), "resolution none") {
		t.Fatalf("unsafe open blocker resolution was accepted: %v", err)
	}
}

func TestCardReplacementRejectsInvalidDirectInput(t *testing.T) {
	state := NewTemplate("PROJECT_STATE.md", "Test", "Test direct card validation.", GitSnapshot{}, "sha256:test")
	verified := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	if err := state.ReplaceFactCards([]FactCard{{
		Status:     "fresh",
		ID:         "F-?",
		Source:     "D-001",
		Paths:      []string{"internal"},
		VerifiedAt: verified,
		Evidence:   "Reviewed input.",
		Summary:    "Invalid identifier must not be rendered.",
	}}); err == nil || !strings.Contains(err.Error(), "fact ID") {
		t.Fatalf("invalid direct fact card was accepted: %v", err)
	}
	if err := state.ReplaceBlockerCards([]BlockerCard{{
		Status:     "open",
		ID:         "B-?",
		TaskID:     "T-001",
		Scope:      []string{"internal"},
		Hypothesis: "Input is invalid.",
		Evidence:   "Fixture reproduces it.",
		NextAction: "Replace the malformed identifier.",
		Resolution: "none",
	}}); err == nil || !strings.Contains(err.Error(), "blocker ID") {
		t.Fatalf("invalid direct blocker card was accepted: %v", err)
	}
	if err := state.ReplaceFactCards([]FactCard{{
		Status:     "fresh",
		ID:         "F-002",
		Source:     "D-001",
		Paths:      []string{"../outside"},
		VerifiedAt: verified,
		Evidence:   "Reviewed input.",
		Summary:    "Escaping path must not be rendered.",
	}}); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escaping direct fact path was accepted: %v", err)
	}
}
