package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHooksDeliverDecisionsAtStartAndOncePerSessionOnFileUse(t *testing.T) {
	root := decisionRepo(t)
	start := runHook(t, "claude", "session-start", map[string]any{"session_id": "s1", "cwd": root, "source": "startup"})
	for _, want := range []string{"Current decisions in docs/adr", "- Use SQLite for local storage → docs/adr/2026-02-10-use-sqlite.md", "- Keep API fields in snake_case"} {
		if !strings.Contains(start, want) {
			t.Errorf("session-start context lacks %q:\n%s", want, start)
		}
	}
	if strings.Contains(start, "Postgres") {
		t.Fatalf("session-start context listed a superseded decision:\n%s", start)
	}

	read := map[string]any{"session_id": "s1", "cwd": root, "tool_name": "Read", "tool_input": map[string]any{"file_path": filepath.Join(root, "internal", "db", "store.go")}}
	first := runHook(t, "claude", "posttool", read)
	if !strings.Contains(first, "Decisions recorded for internal/db/store.go:\n- Use SQLite for local storage") || !strings.Contains(first, "Do not open the database file twice") {
		t.Fatalf("first use of a governed file must show its decision and pitfall:\n%s", first)
	}
	if again := runHook(t, "claude", "posttool", read); again != "" {
		t.Fatalf("a session must not receive the same linked records twice:\n%s", again)
	}
	runHook(t, "claude", "session-start", map[string]any{"session_id": "s1", "cwd": root, "source": "compact"})
	if afterCompaction := runHook(t, "claude", "posttool", read); !strings.Contains(afterCompaction, "Use SQLite for local storage") {
		t.Fatalf("compaction drops earlier context, so linked records must return:\n%s", afterCompaction)
	}

	patch := map[string]any{"session_id": "c1", "cwd": root, "tool_name": "apply_patch", "tool_input": map[string]any{"command": "*** Begin Patch\n*** Update File: internal/db/store.go\n@@\n-package db\n+package db // storage\n*** End Patch\n"}}
	if codex := runHook(t, "codex", "posttool", patch); !strings.Contains(codex, "Use SQLite for local storage") {
		t.Fatalf("a Codex patch must surface the decisions linked to the patched file:\n%s", codex)
	}
}

// runHook sends input to a hook phase the way a harness does and returns the
// additionalContext it emits, or "" when it emits nothing.
func runHook(t *testing.T, harness, phase string, input map[string]any) string {
	t.Helper()
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	writer.Close()
	original := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = original }()
	output := captureStdout(t, func() error { return hookCommand([]string{harness, phase}) })
	if strings.TrimSpace(output) == "" {
		return ""
	}
	var emitted struct {
		HookSpecificOutput struct {
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(output), &emitted); err != nil {
		t.Fatalf("hook output is not JSON: %v\n%s", err, output)
	}
	return emitted.HookSpecificOutput.AdditionalContext
}
