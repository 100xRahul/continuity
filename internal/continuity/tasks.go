package continuity

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var taskPattern = regexp.MustCompile(`(?m)^- \[([ x>])\] ([A-Za-z][A-Za-z0-9_-]*) \| ([^|]+) \| depends: ([^|]+) \| autonomous: (yes|no) \| accepts: (.+)\s*$`)

type Task struct {
	Status     string
	ID         string
	Title      string
	DependsOn  []string
	Autonomous bool
	Acceptance string
}

func ParseTasks(content string) ([]Task, error) {
	matches := taskPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 && strings.TrimSpace(content) != "" {
		return nil, fmt.Errorf("Ready Queue must use '- [ ] T-001 | title | depends: none | autonomous: yes | accepts: observable result' lines")
	}
	tasks := make([]Task, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		id := match[2]
		if seen[id] {
			return nil, fmt.Errorf("duplicate task ID %s", id)
		}
		seen[id] = true
		dependencies := parseDependencies(match[4])
		tasks = append(tasks, Task{
			Status:     match[1],
			ID:         id,
			Title:      strings.TrimSpace(match[3]),
			DependsOn:  dependencies,
			Autonomous: match[5] == "yes",
			Acceptance: strings.TrimSpace(match[6]),
		})
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
	depends := "none"
	if len(task.DependsOn) > 0 {
		depends = strings.Join(task.DependsOn, ", ")
	}
	autonomous := "no"
	if task.Autonomous {
		autonomous = "yes"
	}
	line := fmt.Sprintf("- [ ] %s | %s | depends: %s | autonomous: %s | accepts: %s", task.ID, task.Title, depends, autonomous, task.Acceptance)
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

func UpdateTaskStatus(content, id, status string) (string, error) {
	if status != " " && status != "x" && status != ">" {
		return "", fmt.Errorf("invalid task status %q", status)
	}
	updated := false
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		match := taskPattern.FindStringSubmatch(line)
		if len(match) > 0 && match[2] == id {
			lines[index] = strings.Replace(line, "["+match[1]+"]", "["+status+"]", 1)
			updated = true
		}
	}
	if !updated {
		return "", fmt.Errorf("task %s is not in Ready Queue", id)
	}
	return strings.Join(lines, "\n"), nil
}
