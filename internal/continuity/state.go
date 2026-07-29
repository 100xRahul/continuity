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
	StateFilename      = "PROJECT_STATE.md"
	DefaultStateBudget = 24 * 1024
)

var RequiredSections = []string{
	"Product Intent",
	"Approved Outcomes",
	"Active Work",
	"Ready Queue",
	"Active Decisions",
	"Constraints and Approval Boundaries",
	"Code Map",
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
	Path     string
	Meta     map[string]string
	Body     string
	Sections map[string]string
}

type Validation struct {
	Errors   []string
	Warnings []string
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
	lines := strings.Split(s.Body, "\n")
	var current string
	var contents []string
	flush := func() {
		if current != "" {
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
	temp := s.Path + ".tmp"
	if err := os.WriteFile(temp, contents, 0o644); err != nil {
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
	if s.Meta["schema"] != "1" {
		result.Errorf("unsupported schema %q; expected 1", s.Meta["schema"])
	}
	for _, section := range RequiredSections {
		if _, present := s.Sections[section]; !present {
			result.Errorf("required section %q is missing", section)
		}
	}
	if len(s.Render()) > budget {
		result.Errorf("state is %d bytes; budget is %d bytes", len(s.Render()), budget)
	}
	if active := strings.TrimSpace(s.Meta["active_task"]); active != "" && active != "none" {
		if !strings.Contains(s.Section("Active Work"), "Task: "+active) {
			result.Errorf("active_task %q does not match Active Work", active)
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
	sections := []string{"Active Decisions", "Code Map", "Known Pitfalls and Failed Approaches", "Active Work"}
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
	sort.Strings(paths)
	return paths
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
		"schema":      "1",
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

- Define approved outcomes here. Agents may decompose approved outcomes into engineering tasks but must not invent product features.

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

## Known Pitfalls and Failed Approaches

- Record only failures that remain relevant. Include the failed approach, evidence, resolution, and affected paths.

## Verification

- Record exact commands and their latest known result.

## Recent Material Changes

- State initialized for %s.

## Handoff

Last completed: state initialization.
Immediate restart action: Read this file, run %s brief, then select authorized work.
`, project, objective, project, "continuity")
	return NewState(path, meta, body)
}
