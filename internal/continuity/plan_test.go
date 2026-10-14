package continuity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanNextAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), PlanFilename)
	write := func(milestones, outcomes string) *Plan {
		t.Helper()
		if err := os.WriteFile(path, []byte("---\nschema: 2\n---\n\n# Test plan\n\n## Intent\n\nShip it.\n\n## Milestones\n\n"+milestones+"\n## Outcomes\n\n"+outcomes), 0o644); err != nil {
			t.Fatal(err)
		}
		plan, err := LoadPlan(path)
		if err != nil {
			t.Fatal(err)
		}
		return plan
	}
	plan := write("- [x] M-001 | title: Core | accepts: Core works.\n- [>] M-002 | title: Sync | accepts: Data syncs.\n",
		"- [x] O-001 | milestone: M-002 | title: Push | accepts: Push works.\n- [ ] O-002 | milestone: M-002 | title: Pull | accepts: Pull works.\n")
	if result := plan.Validate(); !result.OK() {
		t.Fatalf("valid plan rejected: %v", result.Errors)
	}
	milestone, open, err := plan.Next()
	if err != nil || milestone == nil || milestone.ID != "M-002" || len(open) != 1 || open[0].ID != "O-002" {
		t.Fatalf("Next = %#v, %#v, %v", milestone, open, err)
	}
	broken := write("- [>] M-001 | title: A | accepts: A.\n- [>] M-002 | title: B | accepts: B.\n", "- [ ] O-001 | milestone: M-009 | title: C | accepts: C.\n-[ ] O-002 | typo\n")
	report := strings.Join(broken.Validate().Errors, "\n")
	for _, want := range []string{"only one milestone can be active", "outcome line \"-[ ] O-002 | typo\""} {
		if !strings.Contains(report, want) {
			t.Errorf("validation does not report %q:\n%s", want, report)
		}
	}
}
