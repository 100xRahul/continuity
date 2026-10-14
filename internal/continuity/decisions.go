package continuity

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DecisionDir holds the project's architecture decision records. Each decision
// is its own Markdown file, so parallel branches never edit the same lines and
// a record stays readable without Continuity.
const DecisionDir = "docs/adr"

var (
	decisionFilePattern  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}-[a-z0-9][a-z0-9-]*\.md$`)
	decisionFieldPattern = regexp.MustCompile(`^- ([A-Za-z]+): (.*)$`)
	markdownLinkPattern  = regexp.MustCompile(`^\[[^\]]*\]\(([^)]+)\)$`)
	decisionGuideFiles   = map[string]bool{"README.md": true, "template.md": true}
)

// Decision is one architecture decision record. Its status is never written in
// the file: a record is current until a later record names it under
// Supersedes, so history is appended instead of rewritten.
type Decision struct {
	File       string
	Title      string
	Date       time.Time
	Paths      []string
	Supersedes []string
	// SupersededBy is derived by LoadDecisions from other records.
	SupersededBy []string
	Body         string
}

func (d Decision) Current() bool { return len(d.SupersededBy) == 0 }

func (d Decision) ProjectWide() bool {
	for _, path := range d.Paths {
		if path == "." {
			return true
		}
	}
	return false
}

// LoadDecisions reads every record in DecisionDir and links each superseded
// record to its replacement. A malformed record is returned as a problem, not
// dropped silently, and the valid records are still returned so one bad file
// cannot hide every other decision.
func LoadDecisions(root string) ([]Decision, []string, error) {
	directory := filepath.Join(root, filepath.FromSlash(DecisionDir))
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	decisions := []Decision{}
	problems := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || decisionGuideFiles[name] || !strings.HasSuffix(name, ".md") {
			continue
		}
		file := path.Join(DecisionDir, name)
		if !decisionFilePattern.MatchString(name) {
			problems = append(problems, file+": name must be YYYY-MM-DD-short-title.md")
			continue
		}
		contents, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			return nil, nil, err
		}
		decision, err := ParseDecision(file, string(contents))
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		decisions = append(decisions, decision)
	}
	problems = append(problems, linkSupersededDecisions(decisions)...)
	sort.SliceStable(decisions, func(i, j int) bool {
		if !decisions[i].Date.Equal(decisions[j].Date) {
			return decisions[i].Date.Before(decisions[j].Date)
		}
		return decisions[i].File < decisions[j].File
	})
	return decisions, problems, nil
}

// ParseDecision reads the title and the "- Key: value" fields that precede the
// first "##" section. Date and Paths are required; Paths use "." for a
// project-wide decision. Other fields and every section stay free text.
func ParseDecision(file, contents string) (Decision, error) {
	contents = strings.ReplaceAll(contents, "\r\n", "\n")
	lines := strings.Split(contents, "\n")
	index := 0
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	if index == len(lines) || !strings.HasPrefix(lines[index], "# ") {
		return Decision{}, errors.New("the first line must be '# Decision title'")
	}
	decision := Decision{File: file, Title: strings.TrimSpace(strings.TrimPrefix(lines[index], "# ")), Body: contents}
	if decision.Title == "" {
		return Decision{}, errors.New("the title is empty")
	}
	fields := map[string]string{}
	for _, line := range lines[index+1:] {
		if strings.HasPrefix(line, "## ") {
			break
		}
		match := decisionFieldPattern.FindStringSubmatch(strings.TrimRight(line, " \t"))
		if match == nil {
			continue
		}
		key := strings.ToLower(match[1])
		if _, seen := fields[key]; seen {
			return Decision{}, fmt.Errorf("field %s appears more than once", match[1])
		}
		fields[key] = strings.TrimSpace(match[2])
	}
	dateValue, ok := fields["date"]
	if !ok {
		return Decision{}, errors.New("missing '- Date: YYYY-MM-DD'")
	}
	date, err := time.Parse(time.DateOnly, dateValue)
	if err != nil {
		return Decision{}, fmt.Errorf("Date %q must be YYYY-MM-DD", dateValue)
	}
	decision.Date = date
	pathsValue, ok := fields["paths"]
	if !ok {
		return Decision{}, errors.New("missing '- Paths: ...'; use . for a project-wide decision")
	}
	if decision.Paths, err = parseCapsulePaths(pathsValue); err != nil {
		return Decision{}, fmt.Errorf("Paths: %w", err)
	}
	if value, ok := fields["supersedes"]; ok {
		for _, item := range strings.Split(value, ",") {
			target, err := decisionLinkTarget(strings.TrimSpace(item))
			if err != nil {
				return Decision{}, err
			}
			decision.Supersedes = append(decision.Supersedes, target)
		}
	}
	return decision, nil
}

// decisionLinkTarget resolves one Supersedes entry, written as a sibling file
// name or a Markdown link to one, to a repository-relative record path.
func decisionLinkTarget(item string) (string, error) {
	if match := markdownLinkPattern.FindStringSubmatch(item); match != nil {
		item = match[1]
	}
	target := path.Join(DecisionDir, strings.Trim(item, "`"))
	if path.Dir(target) != DecisionDir || !decisionFilePattern.MatchString(path.Base(target)) {
		return "", fmt.Errorf("Supersedes entry %q must name a sibling record such as 2026-01-31-old-title.md", item)
	}
	return target, nil
}

func linkSupersededDecisions(decisions []Decision) []string {
	byFile := map[string]int{}
	for index, decision := range decisions {
		byFile[decision.File] = index
	}
	problems := []string{}
	for index := range decisions {
		for _, target := range decisions[index].Supersedes {
			replaced, exists := byFile[target]
			switch {
			case target == decisions[index].File:
				problems = append(problems, target+": a record cannot supersede itself")
			case !exists:
				problems = append(problems, fmt.Sprintf("%s: supersedes %s, which is not a valid decision record", decisions[index].File, target))
			default:
				decisions[replaced].SupersededBy = append(decisions[replaced].SupersededBy, decisions[index].File)
			}
		}
	}
	// A cycle would mark every record in it superseded and leave no current
	// decision for that subject, so it is reported instead of resolved.
	state := map[string]int{}
	var visit func(string) bool
	visit = func(file string) bool {
		switch state[file] {
		case 1:
			return true
		case 2:
			return false
		}
		state[file] = 1
		for _, target := range decisions[byFile[file]].Supersedes {
			if _, exists := byFile[target]; exists && target != file && visit(target) {
				return true
			}
		}
		state[file] = 2
		return false
	}
	for _, decision := range decisions {
		if state[decision.File] == 0 && visit(decision.File) {
			problems = append(problems, decision.File+": its Supersedes links form a cycle")
		}
	}
	return problems
}

func CurrentDecisions(decisions []Decision) []Decision {
	current := []Decision{}
	for _, decision := range decisions {
		if decision.Current() {
			current = append(current, decision)
		}
	}
	return current
}

// DecisionsForPaths returns current decisions that govern any of the given
// repository-relative paths. Project-wide decisions are excluded because the
// session-start index already lists them.
func DecisionsForPaths(decisions []Decision, paths []string) []Decision {
	matches := []Decision{}
	for _, decision := range decisions {
		if decision.Current() && !decision.ProjectWide() && anyPathsOverlap(decision.Paths, paths) {
			matches = append(matches, decision)
		}
	}
	return matches
}
