package continuity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	PlanFilename      = "PROJECT_PLAN.md"
	PlanHistoryDir    = "PROJECT_HISTORY"
	CurrentPlanSchema = "1"
)

var requiredPlanSections = []string{
	"Intent",
	"Sources",
	"Milestones",
	"Outcomes",
	"Constraints",
	"Assumptions",
	"Decisions",
	"Deltas",
}

var planMetadataOrder = []string{
	"schema",
	"status",
	"active_milestone",
	"approved_at",
	"updated_at",
}

var (
	milestonePattern  = regexp.MustCompile(`(?m)^- \[([ x>])\] (M-[A-Za-z0-9_-]+) \| priority: ([1-9][0-9]*) \| title: ([^|]+) \| accepts: (.+)\s*$`)
	outcomePattern    = regexp.MustCompile(`(?m)^- \[([ x>])\] (O-[A-Za-z0-9_-]+) \| milestone: (M-[A-Za-z0-9_-]+) \| priority: ([1-9][0-9]*) \| title: ([^|]+) \| scope: ([^|]+) \| accepts: (.+)\s*$`)
	sourcePattern     = regexp.MustCompile(`(?m)^- (S-[A-Za-z0-9_-]+) \| role: (product|architecture|behavior|reference) \| authority: (inferred|declared) \| confidence: (high|medium|low) \| ref: ([^|]+) \| summary: (.+)\s*$`)
	constraintPattern = regexp.MustCompile(`(?m)^- (C-[A-Za-z0-9_-]+) \| scope: ([^|]+) \| rule: (.+)\s*$`)
	assumptionPattern = regexp.MustCompile(`(?m)^- (A-[A-Za-z0-9_-]+) \| impact: (low|medium|high) \| statement: (.+)\s*$`)
	decisionPattern   = regexp.MustCompile(`(?m)^- \[([ x])\] (D-[A-Za-z0-9_-]+) \| impact: (low|medium|high) \| outcome: (O-[A-Za-z0-9_-]+|none) \| question: ([^|]+) \| recommendation: ([^|]+) \| evidence: ([^|]+) \| resolution: (.+)\s*$`)
	deltaPattern      = regexp.MustCompile(`(?m)^- \[([ x])\] (DLT-[A-Za-z0-9_-]+) \| impact: (technical|product|priority|constraint) \| summary: ([^|]+) \| approval: (pending|approved)\s*$`)
)

// Plan is the committed detailed delivery contract. Unlike State, it may be
// larger and more descriptive, but its records remain compact and structured
// so every harness can validate the same approval boundaries.
type Plan struct {
	Path              string
	Meta              map[string]string
	Body              string
	Sections          map[string]string
	DuplicateSections []string
}

type Milestone struct {
	Status     string
	ID         string
	Priority   int
	Title      string
	Acceptance string
}

type Outcome struct {
	Status     string
	ID         string
	Milestone  string
	Priority   int
	Title      string
	Scope      []string
	Acceptance string
}

type PlanSource struct {
	ID         string
	Role       string
	Authority  string
	Confidence string
	Reference  string
	Summary    string
}

type PlanConstraint struct {
	ID    string
	Scope []string
	Rule  string
}

type Assumption struct {
	ID        string
	Impact    string
	Statement string
}

type DecisionCard struct {
	Resolved       bool
	ID             string
	Impact         string
	Outcome        string
	Question       string
	Recommendation string
	Evidence       string
	Resolution     string
}

type PlanDelta struct {
	Approved bool
	ID       string
	Impact   string
	Summary  string
}

func NewPlan(path, project, intent string) *Plan {
	if project == "" {
		project = filepath.Base(filepath.Dir(path))
	}
	if intent == "" {
		intent = "Capture the approved product direction before autonomous implementation begins."
	}
	meta := map[string]string{
		"schema":           CurrentPlanSchema,
		"status":           "draft",
		"active_milestone": "none",
		"approved_at":      "none",
		"updated_at":       time.Now().UTC().Format(time.RFC3339),
	}
	body := fmt.Sprintf(`# %s delivery plan

## Intent

%s

## Sources

## Milestones

## Outcomes

## Constraints

## Assumptions

## Decisions

## Deltas
`, project, intent)
	return NewPlanDocument(path, meta, body)
}

func NewPlanDocument(path string, meta map[string]string, body string) *Plan {
	p := &Plan{Path: path, Meta: meta, Body: normalizePlanBody(body)}
	p.parseSections()
	return p
}

func LoadPlan(path string) (*Plan, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(bytes), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, errors.New("PROJECT_PLAN.md must start with YAML-like front matter")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return nil, errors.New("PROJECT_PLAN.md front matter is missing its closing ---")
	}
	end += 4
	meta, err := parsePlanFrontMatter(content[4:end])
	if err != nil {
		return nil, err
	}
	return NewPlanDocument(path, meta, content[end+5:]), nil
}

func parsePlanFrontMatter(input string) (map[string]string, error) {
	meta := map[string]string{}
	for index, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid plan front matter at line %d", index+1)
		}
		key = strings.TrimSpace(key)
		if _, exists := meta[key]; exists {
			return nil, fmt.Errorf("duplicate plan front matter field %q", key)
		}
		meta[key] = strings.Trim(strings.TrimSpace(value), "\"")
	}
	return meta, nil
}

func normalizePlanBody(body string) string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if body == "" {
		return ""
	}
	return body + "\n"
}

func (p *Plan) parseSections() {
	p.Sections = map[string]string{}
	p.DuplicateSections = nil
	lines := strings.Split(p.Body, "\n")
	var current string
	var contents []string
	flush := func() {
		if current == "" {
			return
		}
		if _, exists := p.Sections[current]; exists {
			p.DuplicateSections = append(p.DuplicateSections, current)
		}
		p.Sections[current] = strings.Trim(strings.Join(contents, "\n"), "\n")
	}
	for _, line := range lines {
		if strings.HasPrefix(line, "## ") && strings.TrimSpace(strings.TrimPrefix(line, "## ")) != "" {
			flush()
			current = strings.TrimSpace(strings.TrimPrefix(line, "## "))
			contents = nil
			continue
		}
		if current != "" {
			contents = append(contents, line)
		}
	}
	flush()
}

func (p *Plan) Section(name string) string { return p.Sections[name] }

func (p *Plan) ReplaceSection(name, content string) error {
	needle := "## " + name + "\n"
	start := strings.Index(p.Body, needle)
	if start < 0 {
		return fmt.Errorf("plan section %q does not exist", name)
	}
	contentStart := start + len(needle)
	next := strings.Index(p.Body[contentStart:], "\n## ")
	content = strings.TrimSpace(content)
	replacement := needle + "\n"
	if content != "" {
		replacement = needle + content + "\n"
	}
	if next < 0 {
		p.Body = p.Body[:start] + replacement
	} else {
		next += contentStart + 1
		p.Body = p.Body[:start] + replacement + p.Body[next:]
	}
	p.Body = normalizePlanBody(p.Body)
	p.parseSections()
	return nil
}

func (p *Plan) SetMeta(key, value string) {
	if p.Meta == nil {
		p.Meta = map[string]string{}
	}
	p.Meta[key] = value
}

func (p *Plan) Render() string {
	keys := make([]string, 0, len(p.Meta))
	seen := map[string]bool{}
	for _, key := range planMetadataOrder {
		if _, exists := p.Meta[key]; exists {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for key := range p.Meta {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	if len(keys) > len(planMetadataOrder) {
		tail := keys[len(planMetadataOrder):]
		sort.Strings(tail)
		copy(keys[len(planMetadataOrder):], tail)
	}
	var builder strings.Builder
	builder.WriteString("---\n")
	for _, key := range keys {
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(strings.ReplaceAll(p.Meta[key], "\n", " "))
		builder.WriteByte('\n')
	}
	builder.WriteString("---\n\n")
	builder.WriteString(normalizePlanBody(p.Body))
	return builder.String()
}

func (p *Plan) Save() error {
	contents := []byte(p.Render())
	directory := filepath.Dir(p.Path)
	file, err := os.CreateTemp(directory, "."+filepath.Base(p.Path)+"-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, p.Path)
}

func (p *Plan) Validate() Validation {
	result := Validation{}
	for _, key := range planMetadataOrder {
		if strings.TrimSpace(p.Meta[key]) == "" {
			result.Errorf("plan front matter field %q is required", key)
		}
	}
	if p.Meta["schema"] != CurrentPlanSchema {
		result.Errorf("unsupported plan schema %q; expected %s", p.Meta["schema"], CurrentPlanSchema)
	}
	if p.Meta["status"] != "draft" && p.Meta["status"] != "approved" {
		result.Errorf("unsupported plan status %q", p.Meta["status"])
	}
	if approved := p.Meta["approved_at"]; approved != "none" {
		if _, err := time.Parse(time.RFC3339, approved); err != nil {
			result.Errorf("approved_at must be none or RFC3339: %v", err)
		}
	}
	if _, err := time.Parse(time.RFC3339, p.Meta["updated_at"]); err != nil {
		result.Errorf("plan updated_at must be RFC3339: %v", err)
	}
	for _, section := range requiredPlanSections {
		if _, exists := p.Sections[section]; !exists {
			result.Errorf("required plan section %q is missing", section)
		}
	}
	for _, section := range p.DuplicateSections {
		result.Errorf("plan section %q appears more than once", section)
	}
	if strings.TrimSpace(p.Section("Intent")) == "" {
		result.Errorf("plan intent is required")
	}
	milestones, err := p.Milestones()
	if err != nil {
		result.Errorf("milestones: %v", err)
	}
	outcomes, outcomeErr := p.Outcomes()
	if outcomeErr != nil {
		result.Errorf("outcomes: %v", outcomeErr)
	}
	if _, err := p.Sources(); err != nil {
		result.Errorf("sources: %v", err)
	}
	if _, err := p.Constraints(); err != nil {
		result.Errorf("constraints: %v", err)
	}
	if _, err := p.Assumptions(); err != nil {
		result.Errorf("assumptions: %v", err)
	}
	decisions, decisionErr := p.Decisions()
	if decisionErr != nil {
		result.Errorf("decisions: %v", decisionErr)
	}
	if _, err := p.Deltas(); err != nil {
		result.Errorf("deltas: %v", err)
	}
	if err == nil && outcomeErr == nil {
		milestoneIDs := map[string]bool{}
		active := []string{}
		for _, milestone := range milestones {
			milestoneIDs[milestone.ID] = true
			if milestone.Status == ">" {
				active = append(active, milestone.ID)
			}
		}
		if len(active) > 1 {
			result.Errorf("more than one milestone is active: %s", strings.Join(active, ", "))
		}
		for _, outcome := range outcomes {
			if !milestoneIDs[outcome.Milestone] {
				result.Errorf("outcome %s references unknown milestone %s", outcome.ID, outcome.Milestone)
			}
		}
		declared := p.Meta["active_milestone"]
		if declared == "none" {
			if len(active) > 0 {
				result.Errorf("active_milestone is none but milestone %s is active", strings.Join(active, ", "))
			}
		} else if len(active) != 1 || active[0] != declared {
			result.Errorf("active_milestone %q does not match active milestone record", declared)
		}
		if p.Meta["status"] == "approved" && declared == "none" {
			result.Errorf("approved plan must have an active milestone")
		}
		if decisionErr == nil && declared != "none" {
			activeOutcomes := map[string]bool{}
			for _, outcome := range outcomes {
				if outcome.Milestone == declared {
					activeOutcomes[outcome.ID] = true
				}
			}
			for _, decision := range decisions {
				if !decision.Resolved && decision.Impact == "high" && (decision.Outcome == "none" || activeOutcomes[decision.Outcome]) {
					result.Warnf("high-impact decision %s remains open", decision.ID)
				}
			}
		}
	}
	return result
}

func (p *Plan) Milestones() ([]Milestone, error) {
	return parseMilestones(p.Section("Milestones"))
}

func (p *Plan) Outcomes() ([]Outcome, error) {
	return parseOutcomes(p.Section("Outcomes"))
}

func (p *Plan) Sources() ([]PlanSource, error) {
	return parseSources(p.Section("Sources"))
}

func (p *Plan) Constraints() ([]PlanConstraint, error) {
	return parseConstraints(p.Section("Constraints"))
}

func (p *Plan) Assumptions() ([]Assumption, error) {
	return parseAssumptions(p.Section("Assumptions"))
}

func (p *Plan) Decisions() ([]DecisionCard, error) {
	return parseDecisions(p.Section("Decisions"))
}

func (p *Plan) Deltas() ([]PlanDelta, error) {
	return parseDeltas(p.Section("Deltas"))
}

func parseMilestones(content string) ([]Milestone, error) {
	matches, err := allPlanMatches(content, milestonePattern, "milestone")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]Milestone, 0, len(matches))
	for _, match := range matches {
		if seen[match[2]] {
			return nil, fmt.Errorf("duplicate milestone ID %s", match[2])
		}
		seen[match[2]] = true
		priority, err := parsePositiveInt(match[3])
		if err != nil {
			return nil, fmt.Errorf("milestone %s priority: %w", match[2], err)
		}
		items = append(items, Milestone{Status: match[1], ID: match[2], Priority: priority, Title: strings.TrimSpace(match[4]), Acceptance: strings.TrimSpace(match[5])})
	}
	return items, nil
}

func parseOutcomes(content string) ([]Outcome, error) {
	matches, err := allPlanMatches(content, outcomePattern, "outcome")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]Outcome, 0, len(matches))
	for _, match := range matches {
		if seen[match[2]] {
			return nil, fmt.Errorf("duplicate outcome ID %s", match[2])
		}
		seen[match[2]] = true
		priority, err := parsePositiveInt(match[4])
		if err != nil {
			return nil, fmt.Errorf("outcome %s priority: %w", match[2], err)
		}
		scope, err := parsePlanPaths(match[6])
		if err != nil {
			return nil, fmt.Errorf("outcome %s scope: %w", match[2], err)
		}
		items = append(items, Outcome{Status: match[1], ID: match[2], Milestone: match[3], Priority: priority, Title: strings.TrimSpace(match[5]), Scope: scope, Acceptance: strings.TrimSpace(match[7])})
	}
	return items, nil
}

func parseSources(content string) ([]PlanSource, error) {
	matches, err := allPlanMatches(content, sourcePattern, "source")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]PlanSource, 0, len(matches))
	for _, match := range matches {
		if seen[match[1]] {
			return nil, fmt.Errorf("duplicate source ID %s", match[1])
		}
		seen[match[1]] = true
		items = append(items, PlanSource{ID: match[1], Role: match[2], Authority: match[3], Confidence: match[4], Reference: strings.TrimSpace(match[5]), Summary: strings.TrimSpace(match[6])})
	}
	return items, nil
}

func parseConstraints(content string) ([]PlanConstraint, error) {
	matches, err := allPlanMatches(content, constraintPattern, "constraint")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]PlanConstraint, 0, len(matches))
	for _, match := range matches {
		if seen[match[1]] {
			return nil, fmt.Errorf("duplicate constraint ID %s", match[1])
		}
		seen[match[1]] = true
		scope, err := parsePlanPaths(match[2])
		if err != nil {
			return nil, fmt.Errorf("constraint %s scope: %w", match[1], err)
		}
		items = append(items, PlanConstraint{ID: match[1], Scope: scope, Rule: strings.TrimSpace(match[3])})
	}
	return items, nil
}

func parseAssumptions(content string) ([]Assumption, error) {
	matches, err := allPlanMatches(content, assumptionPattern, "assumption")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]Assumption, 0, len(matches))
	for _, match := range matches {
		if seen[match[1]] {
			return nil, fmt.Errorf("duplicate assumption ID %s", match[1])
		}
		seen[match[1]] = true
		items = append(items, Assumption{ID: match[1], Impact: match[2], Statement: strings.TrimSpace(match[3])})
	}
	return items, nil
}

func parseDecisions(content string) ([]DecisionCard, error) {
	matches, err := allPlanMatches(content, decisionPattern, "decision")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]DecisionCard, 0, len(matches))
	for _, match := range matches {
		if seen[match[2]] {
			return nil, fmt.Errorf("duplicate decision ID %s", match[2])
		}
		seen[match[2]] = true
		resolved := match[1] == "x"
		resolution := strings.TrimSpace(match[8])
		if resolved && strings.EqualFold(resolution, "none") {
			return nil, fmt.Errorf("decision %s is resolved but has no resolution", match[2])
		}
		items = append(items, DecisionCard{Resolved: resolved, ID: match[2], Impact: match[3], Outcome: match[4], Question: strings.TrimSpace(match[5]), Recommendation: strings.TrimSpace(match[6]), Evidence: strings.TrimSpace(match[7]), Resolution: resolution})
	}
	return items, nil
}

func parseDeltas(content string) ([]PlanDelta, error) {
	matches, err := allPlanMatches(content, deltaPattern, "delta")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	items := make([]PlanDelta, 0, len(matches))
	for _, match := range matches {
		if seen[match[2]] {
			return nil, fmt.Errorf("duplicate delta ID %s", match[2])
		}
		seen[match[2]] = true
		approved := match[1] == "x"
		if approved != (match[5] == "approved") {
			return nil, fmt.Errorf("delta %s status and approval disagree", match[2])
		}
		items = append(items, PlanDelta{Approved: approved, ID: match[2], Impact: match[3], Summary: strings.TrimSpace(match[4])})
	}
	return items, nil
}

func allPlanMatches(content string, pattern *regexp.Regexp, name string) ([][]string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	matches := pattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("%s section has invalid record syntax", name)
	}
	matched := make([]string, 0, len(matches))
	for _, match := range matches {
		matched = append(matched, strings.TrimSpace(match[0]))
	}
	if strings.Join(matched, "\n") != content {
		return nil, fmt.Errorf("%s section has invalid record syntax", name)
	}
	return matches, nil
}

func parsePositiveInt(value string) (int, error) {
	var number int
	if _, err := fmt.Sscanf(value, "%d", &number); err != nil || number <= 0 {
		return 0, errors.New("must be a positive integer")
	}
	return number, nil
}

func parsePlanPaths(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("must name at least one repository-relative path")
	}
	seen := map[string]bool{}
	paths := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.Trim(strings.TrimSpace(item), "`")
		if item == "" || filepath.IsAbs(item) {
			return nil, fmt.Errorf("invalid path %q", item)
		}
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("path %q escapes the repository", item)
		}
		if !seen[clean] {
			seen[clean] = true
			paths = append(paths, clean)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func RenderMilestone(item Milestone) string {
	return fmt.Sprintf("- [%s] %s | priority: %d | title: %s | accepts: %s", item.Status, item.ID, item.Priority, item.Title, item.Acceptance)
}

func RenderOutcome(item Outcome) string {
	return fmt.Sprintf("- [%s] %s | milestone: %s | priority: %d | title: %s | scope: %s | accepts: %s", item.Status, item.ID, item.Milestone, item.Priority, item.Title, strings.Join(item.Scope, ", "), item.Acceptance)
}

func RenderSource(item PlanSource) string {
	return fmt.Sprintf("- %s | role: %s | authority: %s | confidence: %s | ref: %s | summary: %s", item.ID, item.Role, item.Authority, item.Confidence, item.Reference, item.Summary)
}

func RenderConstraint(item PlanConstraint) string {
	return fmt.Sprintf("- %s | scope: %s | rule: %s", item.ID, strings.Join(item.Scope, ", "), item.Rule)
}

func RenderAssumption(item Assumption) string {
	return fmt.Sprintf("- %s | impact: %s | statement: %s", item.ID, item.Impact, item.Statement)
}

func RenderDecision(item DecisionCard) string {
	status := " "
	if item.Resolved {
		status = "x"
	}
	return fmt.Sprintf("- [%s] %s | impact: %s | outcome: %s | question: %s | recommendation: %s | evidence: %s | resolution: %s", status, item.ID, item.Impact, item.Outcome, item.Question, item.Recommendation, item.Evidence, item.Resolution)
}

func RenderDelta(item PlanDelta) string {
	status, approval := " ", "pending"
	if item.Approved {
		status, approval = "x", "approved"
	}
	return fmt.Sprintf("- [%s] %s | impact: %s | summary: %s | approval: %s", status, item.ID, item.Impact, item.Summary, approval)
}

func AppendPlanRecord(content, line string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return line
	}
	return content + "\n" + line
}

func ReplacePlanRecord(content, id string, replacement string) (string, error) {
	lines := strings.Split(strings.TrimSpace(content), "\n")
	updated := false
	for index, line := range lines {
		if strings.Contains(line, "] "+id+" |") || strings.Contains(line, "- "+id+" |") {
			if updated {
				return "", fmt.Errorf("plan record %s appears more than once", id)
			}
			lines[index] = replacement
			updated = true
		}
	}
	if !updated {
		return "", fmt.Errorf("plan record %s was not found", id)
	}
	return strings.Join(lines, "\n"), nil
}

func PlanPath(root string) string { return filepath.Join(root, PlanFilename) }
