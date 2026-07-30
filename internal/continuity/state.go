package continuity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	StateFilename               = "PROJECT_STATE.md"
	CurrentStateSchema          = "2"
	ApproximateStateTokenBudget = 30_000
	ApproximateBytesPerToken    = 4
	DefaultStateBudget          = ApproximateStateTokenBudget * ApproximateBytesPerToken
)

var RequiredSections = []string{
	"Product Intent",
	"Approved Outcomes",
	"Active Work",
	"Ready Queue",
	"Active Decisions",
	"Constraints and Approval Boundaries",
	"Code Map",
	"Context Capsules",
	"Known Pitfalls and Failed Approaches",
	"Verification",
	"Recent Material Changes",
	"Handoff",
}

var metadataOrder = []string{
	"schema",
	"fingerprint",
	"branch",
	"head",
	"status",
	"active_task",
	"updated_at",
}

type State struct {
	Path              string
	Meta              map[string]string
	Body              string
	Sections          map[string]string
	DuplicateSections []string
}

type Validation struct {
	Errors   []string
	Warnings []string
}

// ContextCapsule is a path-indexed, durable piece of project knowledge. It
// keeps a larger project index navigable: a session receives only capsules
// relevant to its claimed or requested scope rather than the entire state.
type ContextCapsule struct {
	Name    string
	Paths   []string
	Summary string
	Details string
}

func (v Validation) OK() bool { return len(v.Errors) == 0 }

func (v *Validation) Errorf(format string, args ...any) {
	v.Errors = append(v.Errors, fmt.Sprintf(format, args...))
}

func (v *Validation) Warnf(format string, args ...any) {
	v.Warnings = append(v.Warnings, fmt.Sprintf(format, args...))
}

func NewState(path string, meta map[string]string, body string) *State {
	s := &State{Path: path, Meta: meta, Body: normalizeBody(body)}
	s.parseSections()
	return s
}

func LoadState(path string) (*State, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	content := strings.ReplaceAll(string(bytes), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, errors.New("PROJECT_STATE.md must start with YAML-like front matter")
	}
	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return nil, errors.New("PROJECT_STATE.md front matter is missing its closing ---")
	}
	end += 4
	meta, err := parseFrontMatter(content[4:end])
	if err != nil {
		return nil, err
	}
	return NewState(path, meta, content[end+5:]), nil
}

func parseFrontMatter(input string) (map[string]string, error) {
	meta := map[string]string{}
	for index, line := range strings.Split(input, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid front matter at line %d", index+1)
		}
		key = strings.TrimSpace(key)
		if _, exists := meta[key]; exists {
			return nil, fmt.Errorf("duplicate front matter field %q", key)
		}
		value = strings.Trim(strings.TrimSpace(value), "\"")
		meta[key] = value
	}
	return meta, nil
}

func normalizeBody(body string) string {
	body = strings.TrimSpace(strings.ReplaceAll(body, "\r\n", "\n"))
	if body == "" {
		return ""
	}
	return body + "\n"
}

func (s *State) parseSections() {
	s.Sections = map[string]string{}
	s.DuplicateSections = nil
	lines := strings.Split(s.Body, "\n")
	var current string
	var contents []string
	flush := func() {
		if current != "" {
			if _, exists := s.Sections[current]; exists {
				s.DuplicateSections = append(s.DuplicateSections, current)
			}
			s.Sections[current] = strings.Trim(strings.Join(contents, "\n"), "\n")
		}
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

func (s *State) Section(name string) string { return s.Sections[name] }

func (s *State) ReplaceSection(name, content string) error {
	needle := "## " + name + "\n"
	start := strings.Index(s.Body, needle)
	if start < 0 {
		return fmt.Errorf("section %q does not exist", name)
	}
	contentStart := start + len(needle)
	next := strings.Index(s.Body[contentStart:], "\n## ")
	content = strings.TrimSpace(content)
	var replacement string
	if content == "" {
		replacement = needle + "\n"
	} else {
		replacement = needle + content + "\n"
	}
	if next < 0 {
		s.Body = s.Body[:start] + replacement
	} else {
		next += contentStart + 1
		s.Body = s.Body[:start] + replacement + s.Body[next:]
	}
	s.Body = normalizeBody(s.Body)
	s.parseSections()
	return nil
}

func (s *State) SetMeta(key, value string) {
	if s.Meta == nil {
		s.Meta = map[string]string{}
	}
	s.Meta[key] = value
}

func (s *State) Render() string {
	keys := make([]string, 0, len(s.Meta))
	seen := map[string]bool{}
	for _, key := range metadataOrder {
		if _, exists := s.Meta[key]; exists {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	for key := range s.Meta {
		if !seen[key] {
			keys = append(keys, key)
		}
	}
	if len(keys) > len(metadataOrder) {
		tail := keys[len(metadataOrder):]
		sort.Strings(tail)
		copy(keys[len(metadataOrder):], tail)
	}
	var builder strings.Builder
	builder.WriteString("---\n")
	for _, key := range keys {
		value := strings.ReplaceAll(s.Meta[key], "\n", " ")
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(value)
		builder.WriteByte('\n')
	}
	builder.WriteString("---\n\n")
	builder.WriteString(normalizeBody(s.Body))
	return builder.String()
}

func (s *State) Save() error {
	contents := []byte(s.Render())
	directory := filepath.Dir(s.Path)
	file, err := os.CreateTemp(directory, "."+filepath.Base(s.Path)+"-*")
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
	return os.Rename(temp, s.Path)
}

func (s *State) Validate(budget int) Validation {
	if budget <= 0 {
		budget = DefaultStateBudget
	}
	result := Validation{}
	for _, key := range metadataOrder {
		if strings.TrimSpace(s.Meta[key]) == "" {
			result.Errorf("front matter field %q is required", key)
		}
	}
	if s.Meta["schema"] != CurrentStateSchema {
		result.Errorf("unsupported schema %q; expected %s", s.Meta["schema"], CurrentStateSchema)
	}
	switch s.Meta["status"] {
	case "ready", "working", "blocked":
	default:
		result.Errorf("unsupported status %q", s.Meta["status"])
	}
	if _, err := time.Parse(time.RFC3339, s.Meta["updated_at"]); err != nil {
		result.Errorf("updated_at must be RFC3339: %v", err)
	}
	for _, section := range RequiredSections {
		if _, present := s.Sections[section]; !present {
			result.Errorf("required section %q is missing", section)
		}
	}
	for _, section := range s.DuplicateSections {
		result.Errorf("section %q appears more than once", section)
	}
	if _, err := s.ContextCapsules(); err != nil {
		result.Errorf("context capsules: %v", err)
	}
	if len(s.Render()) > budget {
		result.Errorf("state is %d bytes; budget is %d bytes", len(s.Render()), budget)
	}
	if active := strings.TrimSpace(s.Meta["active_task"]); active != "" && active != "none" {
		if !strings.Contains(s.Section("Active Work"), "Task: "+active) {
			result.Errorf("active_task %q does not match Active Work", active)
		}
	}
	tasks, taskErr := ParseTasks(s.Section("Ready Queue"))
	if taskErr == nil {
		active := []string{}
		for _, task := range tasks {
			if task.Status == ">" {
				active = append(active, task.ID)
			}
		}
		declared := strings.TrimSpace(s.Meta["active_task"])
		if declared == "" || declared == "none" {
			if len(active) > 0 {
				result.Errorf("active_task is none but Ready Queue marks %s active", strings.Join(active, ", "))
			}
		} else if len(active) != 1 || active[0] != declared {
			result.Errorf("active_task %q does not match Ready Queue active marker", declared)
		}
	}
	for _, issue := range AnalyzeTasks(s.Section("Ready Queue")) {
		result.Errorf("task queue: %s", issue)
	}
	return result
}

// EvidencePaths returns repository-relative file or directory candidates written
// in backticks in the durable sections where an agent relies on them as evidence.
// Existence and change detection are deliberately left to the Git layer.
func (s *State) EvidencePaths() []string {
	sections := []string{"Active Decisions", "Code Map", "Context Capsules", "Known Pitfalls and Failed Approaches", "Active Work"}
	seen := map[string]bool{}
	paths := []string{}
	for _, section := range sections {
		fragments := strings.Split(s.Section(section), "`")
		for index := 1; index < len(fragments); index += 2 {
			fragment := fragments[index]
			if !looksLikePath(fragment) {
				continue
			}
			path := filepath.ToSlash(filepath.Clean(fragment))
			if !seen[path] {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	if capsules, err := s.ContextCapsules(); err == nil {
		for _, capsule := range capsules {
			for _, path := range capsule.Paths {
				if path == "." {
					continue
				}
				if !seen[path] {
					seen[path] = true
					paths = append(paths, path)
				}
			}
		}
	}
	sort.Strings(paths)
	return paths
}

// ContextCapsules parses the Context Capsules section. Each capsule has a
// level-three heading and exactly one Paths and Summary line:
//
//	### API boundary
//	Paths: api, internal/http
//	Summary: Stable request/response contract.
//	Details: Load only when changing those paths.
func (s *State) ContextCapsules() ([]ContextCapsule, error) {
	content := strings.TrimSpace(s.Section("Context Capsules"))
	if content == "" {
		return nil, nil
	}
	var capsules []ContextCapsule
	var current *ContextCapsule
	var details []string
	flush := func() error {
		if current == nil {
			return nil
		}
		if current.Name == "" {
			return errors.New("capsule heading is empty")
		}
		if len(current.Paths) == 0 {
			return fmt.Errorf("capsule %q is missing Paths", current.Name)
		}
		if strings.TrimSpace(current.Summary) == "" {
			return fmt.Errorf("capsule %q is missing Summary", current.Name)
		}
		current.Details = strings.Trim(strings.Join(details, "\n"), "\n")
		capsules = append(capsules, *current)
		current = nil
		details = nil
		return nil
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, err
			}
			name := strings.TrimSpace(strings.TrimPrefix(line, "### "))
			if name == "" {
				return nil, errors.New("capsule heading is empty")
			}
			if seen[name] {
				return nil, fmt.Errorf("duplicate capsule %q", name)
			}
			seen[name] = true
			current = &ContextCapsule{Name: name}
			continue
		}
		if current == nil {
			if strings.TrimSpace(line) != "" {
				return nil, errors.New("capsules must begin with a ### heading")
			}
			continue
		}
		if strings.HasPrefix(line, "Paths:") {
			if len(current.Paths) > 0 {
				return nil, fmt.Errorf("capsule %q has more than one Paths line", current.Name)
			}
			paths, err := parseCapsulePaths(strings.TrimSpace(strings.TrimPrefix(line, "Paths:")))
			if err != nil {
				return nil, fmt.Errorf("capsule %q paths: %w", current.Name, err)
			}
			current.Paths = paths
			continue
		}
		if strings.HasPrefix(line, "Summary:") {
			if current.Summary != "" {
				return nil, fmt.Errorf("capsule %q has more than one Summary line", current.Name)
			}
			current.Summary = strings.TrimSpace(strings.TrimPrefix(line, "Summary:"))
			continue
		}
		details = append(details, line)
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return capsules, nil
}

func parseCapsulePaths(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("Paths must name at least one repository-relative path")
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

// SelectContextCapsules ranks capsules by overlap with repository-relative
// scopes. A project-wide '.' capsule is retained as low-priority orientation;
// exact and nested path matches come first. With no scope, only project-wide
// capsules are selected.
func SelectContextCapsules(capsules []ContextCapsule, scopes []string) []ContextCapsule {
	type candidate struct {
		capsule ContextCapsule
		score   int
	}
	candidates := []candidate{}
	for _, capsule := range capsules {
		score := 0
		for _, path := range capsule.Paths {
			if len(scopes) == 0 {
				if path == "." {
					score = max(score, 1)
				}
				continue
			}
			for _, scope := range scopes {
				scope = filepath.ToSlash(filepath.Clean(scope))
				switch {
				case path == ".":
					score = max(score, 1)
				case scope == ".":
					score = max(score, 2)
				case path == scope:
					score = max(score, 4)
				case strings.HasPrefix(path, scope+"/") || strings.HasPrefix(scope, path+"/"):
					score = max(score, 3)
				}
			}
		}
		if score > 0 {
			candidates = append(candidates, candidate{capsule: capsule, score: score})
		}
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		return candidates[i].capsule.Name < candidates[j].capsule.Name
	})
	selected := make([]ContextCapsule, 0, len(candidates))
	for _, candidate := range candidates {
		selected = append(selected, candidate.capsule)
	}
	return selected
}

func (c ContextCapsule) Render() string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Context capsule: %s\n", c.Name)
	fmt.Fprintf(&builder, "Paths: %s\n", strings.Join(c.Paths, ","))
	fmt.Fprintf(&builder, "Summary: %s\n", c.Summary)
	if strings.TrimSpace(c.Details) != "" {
		builder.WriteString(c.Details)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func looksLikePath(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\n\r") || strings.HasPrefix(value, "-") {
		return false
	}
	return strings.Contains(value, "/") || strings.Contains(filepath.Base(value), ".")
}

func NewTemplate(path, project, objective string, snapshot GitSnapshot, fingerprint string) *State {
	if project == "" {
		project = filepath.Base(filepath.Dir(path))
	}
	if objective == "" {
		objective = "Record the first approved engineering objective before autonomous work begins."
	}
	head := snapshot.Head
	if head == "" {
		head = "unborn"
	}
	branch := snapshot.Branch
	if branch == "" {
		branch = "unborn"
	}
	meta := map[string]string{
		"schema":      CurrentStateSchema,
		"fingerprint": fingerprint,
		"branch":      branch,
		"head":        head,
		"status":      "ready",
		"active_task": "none",
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	body := fmt.Sprintf(`# %s

## Product Intent

%s

## Approved Outcomes

- Delivery plan draft: PROJECT_PLAN.md. No implementation milestone is executable until the user explicitly approves one.

## Active Work

Task: none
Status: ready
Mode: strict
Goal: none
Acceptance criteria: none
Immediate next action: Select an approved ready task or request product direction.
Edit scope: none
Validation: none

## Ready Queue

- [ ] T-001 | Define the first approved engineering task | depends: none | autonomous: no | accepts: User-approved goal and observable completion criteria recorded.

## Active Decisions

- Add only active engineering decisions with a short why and a repository-relative evidence anchor.

## Constraints and Approval Boundaries

- Do not invent product outcomes.
- Request approval for destructive actions, production changes, credentials, paid services, or unresolved product choices.

## Code Map

- Add subsystem, responsibility, repository-relative entry points, and tests as the project becomes known.

## Context Capsules

%s

## Known Pitfalls and Failed Approaches

- Record only failures that remain relevant. Include the failed approach, evidence, resolution, and affected paths.

## Verification

- Record exact commands and their latest known result.

## Recent Material Changes

- State initialized for %s.

## Handoff

Last completed: state initialization.
Immediate restart action: Run %s context and %s status, complete the planning interview, and wait for explicit plan approval before claiming implementation work.
`, project, objective, defaultContextCapsules, project, "continuity", "continuity")
	return NewState(path, meta, body)
}

const defaultContextCapsules = `### Project-wide orientation
Paths: .
Summary: Durable context is indexed by repository path so a new session can load only what it needs.
Details: Add short, factual capsules for subsystems or decisions that need more detail than the root index. Do not paste chat transcripts, reasoning traces, or source code.`
