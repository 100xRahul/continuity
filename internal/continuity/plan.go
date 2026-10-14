package continuity

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	PlanFilename      = "PROJECT_PLAN.md"
	CurrentPlanSchema = "2"
)

var (
	planSections     = []string{"Intent", "Milestones", "Outcomes"}
	milestonePattern = regexp.MustCompile(`^- \[([ >x])\] (M-[A-Za-z0-9_-]+) \| title: ([^|]+) \| accepts: (.+)$`)
	outcomePattern   = regexp.MustCompile(`^- \[([ x])\] (O-[A-Za-z0-9_-]+) \| milestone: (M-[A-Za-z0-9_-]+) \| title: ([^|]+) \| accepts: (.+)$`)
)

// Plan is the user's roadmap: why the product exists, its milestones in
// delivery order, and the outcomes each milestone must deliver. Agents read it
// to know what comes next without being told; only the user changes what it
// contains, while agents mark outcomes done.
type Plan struct {
	Path       string
	Schema     string
	Sections   map[string]string
	Duplicates []string
}

// Milestone status is " " (planned), ">" (active), or "x" (done). At most one
// milestone is active.
type Milestone struct {
	Status  string
	ID      string
	Title   string
	Accepts string
}

type Outcome struct {
	Done      bool
	ID        string
	Milestone string
	Title     string
	Accepts   string
}

func PlanPath(root string) string { return filepath.Join(root, PlanFilename) }

func LoadPlan(path string) (*Plan, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	meta, body, err := splitFrontMatter(string(bytes), filepath.Base(path))
	if err != nil {
		return nil, err
	}
	sections, duplicates := markdownSections(body)
	return &Plan{Path: path, Schema: meta["schema"], Sections: sections, Duplicates: duplicates}, nil
}

// NewPlanText is the plan that init writes before the user's first interview.
func NewPlanText(project, intent string) string {
	if intent == "" {
		intent = "State why this product exists and who it serves before the first milestone is planned."
	}
	return fmt.Sprintf(`---
schema: %s
---

# %s plan

## Intent

%s

## Milestones

## Outcomes
`, CurrentPlanSchema, project, intent)
}

func (p *Plan) Intent() string { return strings.TrimSpace(p.Sections["Intent"]) }

func (p *Plan) Milestones() ([]Milestone, error) {
	milestones := []Milestone{}
	for _, line := range planRecordLines(p.Sections["Milestones"]) {
		match := milestonePattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("milestone line %q must be '- [ ] M-001 | title: ... | accepts: ...' with status ' ', '>' (active), or 'x' (done)", line)
		}
		milestones = append(milestones, Milestone{Status: match[1], ID: match[2], Title: strings.TrimSpace(match[3]), Accepts: strings.TrimSpace(match[4])})
	}
	return milestones, nil
}

func (p *Plan) Outcomes() ([]Outcome, error) {
	outcomes := []Outcome{}
	for _, line := range planRecordLines(p.Sections["Outcomes"]) {
		match := outcomePattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("outcome line %q must be '- [ ] O-001 | milestone: M-001 | title: ... | accepts: ...' with status ' ' or 'x'", line)
		}
		outcomes = append(outcomes, Outcome{Done: match[1] == "x", ID: match[2], Milestone: match[3], Title: strings.TrimSpace(match[4]), Accepts: strings.TrimSpace(match[5])})
	}
	return outcomes, nil
}

// planRecordLines returns every line that starts with "-". Each must parse as
// a record, so a typo cannot hide an outcome from the next session.
func planRecordLines(section string) []string {
	lines := []string{}
	for _, line := range strings.Split(section, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "-") {
			lines = append(lines, line)
		}
	}
	return lines
}

func (p *Plan) Validate() Validation {
	result := Validation{}
	if p.Schema != CurrentPlanSchema {
		result.Errorf("unsupported plan schema %q; expected %s with only Intent, Milestones, and Outcomes", p.Schema, CurrentPlanSchema)
	}
	for _, section := range planSections {
		if _, present := p.Sections[section]; !present {
			result.Errorf("plan section %q is missing", section)
		}
	}
	for _, section := range p.Duplicates {
		result.Errorf("plan section %q appears more than once", section)
	}
	milestones, err := p.Milestones()
	if err != nil {
		result.Errorf("%v", err)
	}
	outcomes, err := p.Outcomes()
	if err != nil {
		result.Errorf("%v", err)
	}
	ids := map[string]bool{}
	active := []string{}
	for _, milestone := range milestones {
		if ids[milestone.ID] {
			result.Errorf("duplicate plan ID %s", milestone.ID)
		}
		ids[milestone.ID] = true
		if milestone.Status == ">" {
			active = append(active, milestone.ID)
		}
	}
	if len(active) > 1 {
		result.Errorf("only one milestone can be active; found %s", strings.Join(active, ", "))
	}
	for _, outcome := range outcomes {
		if ids[outcome.ID] {
			result.Errorf("duplicate plan ID %s", outcome.ID)
		}
		ids[outcome.ID] = true
		if !hasMilestone(milestones, outcome.Milestone) {
			result.Errorf("outcome %s names unknown milestone %s", outcome.ID, outcome.Milestone)
		}
	}
	if p.Intent() == "" {
		result.Warnf("plan Intent is empty")
	}
	return result
}

func hasMilestone(milestones []Milestone, id string) bool {
	for _, milestone := range milestones {
		if milestone.ID == id {
			return true
		}
	}
	return false
}

// Next returns the active milestone and its unfinished outcomes in plan order,
// or nil when no milestone is active.
func (p *Plan) Next() (*Milestone, []Outcome, error) {
	milestones, err := p.Milestones()
	if err != nil {
		return nil, nil, err
	}
	outcomes, err := p.Outcomes()
	if err != nil {
		return nil, nil, err
	}
	for _, milestone := range milestones {
		if milestone.Status != ">" {
			continue
		}
		open := []Outcome{}
		for _, outcome := range outcomes {
			if outcome.Milestone == milestone.ID && !outcome.Done {
				open = append(open, outcome)
			}
		}
		return &milestone, open, nil
	}
	return nil, nil, nil
}
