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
