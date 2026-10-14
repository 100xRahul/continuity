package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"continuity/internal/continuity"
)

type hookInput struct {
	SessionID string          `json:"session_id"`
	Cwd       string          `json:"cwd"`
	Source    string          `json:"source"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

var (
	hookHarnesses = map[string]bool{"claude": true, "codex": true}
	hookPhases    = map[string]bool{"session-start": true, "posttool": true}
)

const (
	shownRecordRetention = 7 * 24 * time.Hour
	maxTouchedPaths      = 8
)

// hookCommand adapts a harness lifecycle hook. The harness sends its session
// identity and working directory as JSON on stdin.
func hookCommand(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: continuity hook claude|codex session-start|posttool")
	}
	harness, phase := args[0], args[1]
	if !hookHarnesses[harness] {
		return fmt.Errorf("unknown hook harness %q", harness)
	}
	if !hookPhases[phase] {
		return fmt.Errorf("unknown hook phase %q", phase)
	}
	flags := flag.NewFlagSet("hook "+harness+" "+phase, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", "", "repository path; defaults to the harness working directory")
	if err := flags.Parse(args[2:]); err != nil {
		return err
	}
	input := hookInput{}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	directory := *repo
	if directory == "" {
		directory = input.Cwd
	}
	if directory == "" {
		directory = "."
	}
	root, err := repoRoot(directory)
	if err != nil {
		return nil // Hooks must not interfere outside a repository.
	}
	if _, err := os.Stat(filepath.Join(root, continuity.StateFilename)); errors.Is(err, os.ErrNotExist) {
		return nil // This repository does not use Continuity.
	}
	owner := harness + ":" + strings.TrimSpace(input.SessionID)
	switch phase {
	case "session-start":
		if input.Source == "compact" || input.Source == "clear" {
			if err := continuity.ResetShownRecords(root, owner); err != nil {
				return err
			}
		}
		if err := continuity.PruneShownRecords(root, shownRecordRetention); err != nil {
			return err
		}
		context, err := compactContext(root, sessionStartContextLimit, nil)
		if err != nil {
			// The agent must learn why its context is missing, so the reason
			// replaces the packet instead of disappearing into a hook log.
			context = "Continuity context could not be built: " + err.Error() + "\n"
		}
		return emitHookContext("SessionStart", context)
	default:
		paths := touchedPaths(root, input)
		if len(paths) == 0 {
			return nil
		}
		shown, err := continuity.LoadShownRecords(root, owner)
		if err != nil {
			return err
		}
		report, added, err := pathRecall(root, paths, shown)
		if err != nil || report == "" {
			return err
		}
		if err := continuity.AddShownRecords(root, owner, added); err != nil {
			return err
		}
		return emitHookContext("PostToolUse", report)
	}
}

// touchedPaths returns the repository paths a tool call read or changed.
// Claude Code file tools name them directly, Codex patches name them in their
// headers, and a shell command counts every argument that names an existing
// repository path. Malformed tool input yields no paths: the hook only adds
// optional context and must not fail the tool call it observes.
func touchedPaths(root string, input hookInput) []string {
	var tool struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"`
	}
	if err := json.Unmarshal(input.ToolInput, &tool); err != nil {
		return nil
	}
	candidates := []string{}
	switch {
	case tool.FilePath != "":
		candidates = append(candidates, tool.FilePath)
	case tool.NotebookPath != "":
		candidates = append(candidates, tool.NotebookPath)
	case input.ToolName == "apply_patch":
		candidates = patchPaths(tool.Command)
	case input.ToolName == "Bash":
		candidates = shellPathArguments(tool.Command)
	}
	base := input.Cwd
	if base == "" {
		base = root
	}
	seen := map[string]bool{}
	paths := []string{}
	for _, candidate := range candidates {
		if !filepath.IsAbs(candidate) {
			candidate = filepath.Join(base, candidate)
		}
		if input.ToolName == "Bash" {
			if _, err := os.Stat(candidate); err != nil {
				continue
			}
		}
		path, inside := repositoryPath(root, candidate)
		if !inside || path == "." || seen[path] {
			continue
		}
		seen[path] = true
		paths = append(paths, path)
		if len(paths) == maxTouchedPaths {
			break
		}
	}
	return paths
}

func patchPaths(patch string) []string {
	paths := []string{}
	for _, line := range strings.Split(patch, "\n") {
		for _, prefix := range []string{"*** Add File: ", "*** Update File: ", "*** Delete File: ", "*** Move to: "} {
			if path, found := strings.CutPrefix(strings.TrimSpace(line), prefix); found {
				paths = append(paths, strings.TrimSpace(path))
			}
		}
	}
	return paths
}

func shellPathArguments(command string) []string {
	fields := strings.FieldsFunc(command, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(";|&<>()'\"`", r)
	})
	arguments := []string{}
	for _, field := range fields {
		if !strings.HasPrefix(field, "-") && strings.ContainsAny(field, "/.") {
			arguments = append(arguments, field)
		}
	}
	return arguments
}

func emitHookContext(event, additionalContext string) error {
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": additionalContext,
		},
	})
}
