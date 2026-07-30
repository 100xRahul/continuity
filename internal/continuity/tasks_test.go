package continuity

import (
	"strings"
	"testing"
)

func TestReadyTasksRespectDependenciesAndAuthorization(t *testing.T) {
	queue := strings.Join([]string{
		"- [x] T-001 | completed | depends: none | autonomous: yes | accepts: completed",
		"- [ ] T-002 | ready | depends: T-001 | autonomous: yes | accepts: targeted test passes",
		"- [ ] T-003 | blocked | depends: T-002 | autonomous: yes | accepts: implementation verified",
		"- [ ] T-004 | user decision | depends: none | autonomous: no | accepts: approved direction",
	}, "\n")
	ready, err := ReadyTasks(queue)
	if err != nil {
		t.Fatal(err)
	}
	if len(ready) != 1 || ready[0].ID != "T-002" {
		t.Fatalf("unexpected ready tasks: %#v", ready)
	}
}

func TestAnalyzeTasksFindsCycles(t *testing.T) {
	queue := strings.Join([]string{
		"- [ ] T-001 | first | depends: T-002 | autonomous: yes | accepts: first complete",
		"- [ ] T-002 | second | depends: T-001 | autonomous: yes | accepts: second complete",
	}, "\n")
	warnings := AnalyzeTasks(queue)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "dependency cycle") {
		t.Fatalf("expected cycle warning, got %v", warnings)
	}
}

func TestAddTaskRequiresValidUniqueDependencies(t *testing.T) {
	queue := "- [x] T-001 | completed | depends: none | autonomous: yes | accepts: completed"
	updated, err := AddTask(queue, Task{ID: "T-002", Title: "next work", DependsOn: []string{"T-001"}, Autonomous: true, Acceptance: "focused tests pass"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(updated, "T-002") {
		t.Fatalf("new task missing from queue: %q", updated)
	}
	if _, err := AddTask(updated, Task{ID: "T-002", Title: "duplicate", Autonomous: true, Acceptance: "must fail"}); err == nil || !strings.Contains(err.Error(), "duplicate task ID") {
		t.Fatalf("duplicate task was accepted: %v", err)
	}
	if _, err := AddTask(updated, Task{ID: "T-003", Title: "unknown dependency", DependsOn: []string{"T-404"}, Autonomous: true, Acceptance: "must fail"}); err == nil || !strings.Contains(err.Error(), "unknown task") {
		t.Fatalf("unknown dependency was accepted: %v", err)
	}
}

func TestOutcomeTaskContractRoundTripsWithDerivedRationale(t *testing.T) {
	queue := "- [ ] T-011 | Add task contracts | outcome: O-001 | scope: cmd/continuity, internal/continuity | depends: none | autonomous: yes | validates: go test ./... | accepts: Planned tasks are bounded. | derived: It unblocks safe autonomous continuation."
	tasks, err := ParseTasks(queue)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("task count = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.Outcome != "O-001" || strings.Join(task.Scope, ",") != "cmd/continuity,internal/continuity" || task.Validation != "go test ./..." || task.Rationale != "It unblocks safe autonomous continuation." {
		t.Fatalf("contract did not parse: %#v", task)
	}
	updated, err := AddTask("", task)
	if err != nil {
		t.Fatal(err)
	}
	if updated != queue {
		t.Fatalf("task did not render stably:\nwant %q\n got %q", queue, updated)
	}
}

func TestOutcomeTaskContractRequiresFocusedScopeAndValidation(t *testing.T) {
	if _, err := AddTask("", Task{ID: "T-010", Title: "Stray contract", Scope: []string{"internal/continuity"}, Autonomous: true, Acceptance: "Must fail."}); err == nil || !strings.Contains(err.Error(), "require an outcome") {
		t.Fatalf("contract fields without an outcome were accepted: %v", err)
	}
	if _, err := AddTask("", Task{ID: "T-011", Title: "Missing scope", Outcome: "O-001", Autonomous: true, Validation: "go test ./...", Acceptance: "Must fail."}); err == nil || !strings.Contains(err.Error(), "focused scope") {
		t.Fatalf("missing scope accepted: %v", err)
	}
	if _, err := AddTask("", Task{ID: "T-011", Title: "Missing validation", Outcome: "O-001", Scope: []string{"internal/continuity"}, Autonomous: true, Acceptance: "Must fail."}); err == nil || !strings.Contains(err.Error(), "validation") {
		t.Fatalf("missing validation accepted: %v", err)
	}
	if _, err := ParseTasks("- [ ] T-011 | Unsafe scope | outcome: O-001 | scope: ../outside | depends: none | autonomous: yes | validates: go test ./... | accepts: Must fail."); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escaping task scope accepted: %v", err)
	}
}
