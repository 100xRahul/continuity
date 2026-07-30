package continuity

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	legacyTaskPattern   = regexp.MustCompile(`^- \[([ x>])\] ([A-Za-z][A-Za-z0-9_-]*) \| ([^|]+) \| depends: ([^|]+) \| autonomous: (yes|no) \| accepts: (.+)$`)
	contractTaskPattern = regexp.MustCompile(`^- \[([ x>])\] ([A-Za-z][A-Za-z0-9_-]*) \| ([^|]+) \| outcome: (O-[A-Za-z0-9_-]+) \| scope: ([^|]+) \| depends: ([^|]+) \| autonomous: (yes|no) \| validates: ([^|]+) \| accepts: (.+?)(?: \| derived: (.+))?$`)
)

type Task struct {
	Status     string
	ID         string
	Title      string
	Outcome    string
	Scope      []string
	DependsOn  []string
	Autonomous bool
	Validation string
	Acceptance string
	// Rationale is present only for an agent-derived technical task. It is
	// intentionally compact: the approved Outcome remains the authority for
	// product scope, while this field records why the focused task exists.
	Rationale string
}

func ParseTasks(content string) ([]Task, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	lines := strings.Split(content, "\n")
	tasks := make([]Task, 0, len(lines))
	seen := map[string]bool{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			return nil, errors.New("Ready Queue cannot contain blank lines")
		}
		var task Task
		if match := contractTaskPattern.FindStringSubmatch(line); len(match) > 0 {
			scope, err := parsePlanPaths(match[5])
			if err != nil {
				return nil, fmt.Errorf("task %s scope: %w", match[2], err)
			}
			task = Task{
				Status:     match[1],
				ID:         match[2],
				Title:      strings.TrimSpace(match[3]),
				Outcome:    match[4],
				Scope:      scope,
				DependsOn:  parseDependencies(match[6]),
				Autonomous: match[7] == "yes",
				Validation: strings.TrimSpace(match[8]),
				Acceptance: strings.TrimSpace(match[9]),
				Rationale:  strings.TrimSpace(match[10]),
			}
		} else if match := legacyTaskPattern.FindStringSubmatch(line); len(match) > 0 {
			task = Task{
				Status:     match[1],
				ID:         match[2],
				Title:      strings.TrimSpace(match[3]),
				DependsOn:  parseDependencies(match[4]),
				Autonomous: match[5] == "yes",
				Acceptance: strings.TrimSpace(match[6]),
			}
		} else {
			return nil, fmt.Errorf("Ready Queue must use a legacy task line or '- [ ] T-001 | title | outcome: O-001 | scope: path | depends: none | autonomous: yes | validates: command | accepts: observable result'")
		}
		if err := validateTaskFields(task); err != nil {
			return nil, fmt.Errorf("task %s: %w", task.ID, err)
		}
		id := task.ID
		if seen[id] {
			return nil, fmt.Errorf("duplicate task ID %s", id)
		}
		seen[id] = true
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func parseDependencies(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "none") {
		return nil
	}
	parts := strings.Split(value, ",")
	dependencies := make([]string, 0, len(parts))
	for _, part := range parts {
		if dependency := strings.TrimSpace(part); dependency != "" {
			dependencies = append(dependencies, dependency)
		}
	}
	return dependencies
}

func AnalyzeTasks(content string) []string {
	tasks, err := ParseTasks(content)
	if err != nil {
		return []string{err.Error()}
	}
	byID := map[string]Task{}
	active := 0
	for _, task := range tasks {
		byID[task.ID] = task
		if task.Status == ">" {
			active++
		}
	}
	warnings := []string{}
	if active > 1 {
		warnings = append(warnings, "more than one task is marked active")
	}
	for _, task := range tasks {
		if task.Autonomous && (task.Acceptance == "" || strings.EqualFold(task.Acceptance, "none")) {
			warnings = append(warnings, fmt.Sprintf("%s is autonomous but has no acceptance criteria", task.ID))
		}
		for _, dependency := range task.DependsOn {
			if _, known := byID[dependency]; !known {
				warnings = append(warnings, fmt.Sprintf("%s depends on unknown task %s", task.ID, dependency))
			}
		}
	}
	if cycle := taskCycle(byID); len(cycle) > 0 {
		warnings = append(warnings, "dependency cycle: "+strings.Join(cycle, " -> "))
	}
	return warnings
}

func taskCycle(tasks map[string]Task) []string {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	path := []string{}
	var visit func(string) []string
	visit = func(id string) []string {
		if visiting[id] {
			for index, item := range path {
				if item == id {
					return append(path[index:], id)
				}
			}
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		path = append(path, id)
		for _, dependency := range tasks[id].DependsOn {
			if _, exists := tasks[dependency]; exists {
				if cycle := visit(dependency); len(cycle) > 0 {
					return cycle
				}
			}
		}
		path = path[:len(path)-1]
		visiting[id] = false
		visited[id] = true
		return nil
	}
	ids := make([]string, 0, len(tasks))
	for id := range tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if cycle := visit(id); len(cycle) > 0 {
			return cycle
		}
	}
	return nil
}

func ReadyTasks(content string) ([]Task, error) {
	tasks, err := ParseTasks(content)
	if err != nil {
		return nil, err
	}
	byID := map[string]Task{}
	for _, task := range tasks {
		byID[task.ID] = task
	}
	ready := []Task{}
	for _, task := range tasks {
		if task.Status != " " || !task.Autonomous || task.Acceptance == "" || strings.EqualFold(task.Acceptance, "none") {
			continue
		}
		unblocked := true
		for _, dependency := range task.DependsOn {
			dependentTask, exists := byID[dependency]
			if !exists || dependentTask.Status != "x" {
				unblocked = false
				break
			}
		}
		if unblocked {
			ready = append(ready, task)
		}
	}
	return ready, nil
}

// AddTask appends one queued task after validating its syntax, dependencies,
// and uniqueness against the existing Ready Queue.
func AddTask(content string, task Task) (string, error) {
	if task.Status != "" && task.Status != " " {
		return "", errors.New("new tasks must begin queued")
	}
	task.Status = " "
	if err := validateTaskFields(task); err != nil {
		return "", err
	}
	line := RenderTask(task)
	candidate := strings.TrimSpace(content)
	if candidate != "" {
		candidate += "\n" + line
	} else {
		candidate = line
	}
	if _, err := ParseTasks(candidate); err != nil {
		return "", err
	}
	if issues := AnalyzeTasks(candidate); len(issues) > 0 {
		return "", errors.New(strings.Join(issues, "; "))
	}
	return candidate, nil
}

// RenderTask uses the richer outcome contract whenever an outcome is present;
// old task records remain parseable so a repository can adopt the plan layer
// without rewriting historic task history.
func RenderTask(task Task) string {
	depends := "none"
	if len(task.DependsOn) > 0 {
		depends = strings.Join(task.DependsOn, ", ")
	}
	autonomous := "no"
	if task.Autonomous {
		autonomous = "yes"
	}
	if task.Outcome != "" {
		line := fmt.Sprintf("- [%s] %s | %s | outcome: %s | scope: %s | depends: %s | autonomous: %s | validates: %s | accepts: %s", task.Status, task.ID, task.Title, task.Outcome, strings.Join(task.Scope, ", "), depends, autonomous, task.Validation, task.Acceptance)
		if task.Rationale != "" {
			line += " | derived: " + task.Rationale
		}
		return line
	}
	return fmt.Sprintf("- [%s] %s | %s | depends: %s | autonomous: %s | accepts: %s", task.Status, task.ID, task.Title, depends, autonomous, task.Acceptance)
}

func validateTaskFields(task Task) error {
	for name, value := range map[string]string{
		"task ID":    task.ID,
		"task title": task.Title,
		"acceptance": task.Acceptance,
	} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n|") {
			return fmt.Errorf("%s must be non-empty single-line text without |", name)
		}
	}
	if task.Outcome == "" {
		if len(task.Scope) > 0 || strings.TrimSpace(task.Validation) != "" || strings.TrimSpace(task.Rationale) != "" {
			return fmt.Errorf("task %s contract fields require an outcome", task.ID)
		}
		return nil
	}
	if !strings.HasPrefix(task.Outcome, "O-") {
		return fmt.Errorf("task %s outcome must begin with O-", task.ID)
	}
	if len(task.Scope) == 0 {
		return fmt.Errorf("task %s requires a focused scope", task.ID)
	}
	if strings.TrimSpace(task.Validation) == "" || strings.ContainsAny(task.Validation, "\r\n|") {
		return fmt.Errorf("task %s requires a single-line validation command", task.ID)
	}
	if strings.TrimSpace(task.Rationale) != "" && strings.ContainsAny(task.Rationale, "\r\n|") {
		return fmt.Errorf("task %s rationale must be single-line text without |", task.ID)
	}
	return nil
}

func UpdateTaskStatus(content, id, status string) (string, error) {
	if status != " " && status != "x" && status != ">" {
		return "", fmt.Errorf("invalid task status %q", status)
	}
	updated := false
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		task, ok := parseTaskLine(strings.TrimSpace(line))
		if ok && task.ID == id {
			lines[index] = strings.Replace(line, "["+task.Status+"]", "["+status+"]", 1)
			updated = true
		}
	}
	if !updated {
		return "", fmt.Errorf("task %s is not in Ready Queue", id)
	}
	return strings.Join(lines, "\n"), nil
}

func parseTaskLine(line string) (Task, bool) {
	if match := contractTaskPattern.FindStringSubmatch(line); len(match) > 0 {
		scope, err := parsePlanPaths(match[5])
		if err != nil {
			return Task{}, false
		}
		task := Task{Status: match[1], ID: match[2], Title: strings.TrimSpace(match[3]), Outcome: match[4], Scope: scope, DependsOn: parseDependencies(match[6]), Autonomous: match[7] == "yes", Validation: strings.TrimSpace(match[8]), Acceptance: strings.TrimSpace(match[9]), Rationale: strings.TrimSpace(match[10])}
		return task, validateTaskFields(task) == nil
	}
	if match := legacyTaskPattern.FindStringSubmatch(line); len(match) > 0 {
		task := Task{Status: match[1], ID: match[2], Title: strings.TrimSpace(match[3]), DependsOn: parseDependencies(match[4]), Autonomous: match[5] == "yes", Acceptance: strings.TrimSpace(match[6])}
		return task, validateTaskFields(task) == nil
	}
	return Task{}, false
}
