package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"continuity/internal/continuity"
)

const version = "0.1.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = initCommand(os.Args[2:])
	case "brief":
		err = briefCommand(os.Args[2:])
	case "check", "doctor":
		err = checkCommand(os.Args[2:])
	case "reconcile":
		err = reconcileCommand(os.Args[2:])
	case "sync":
		err = syncCommand(os.Args[2:])
	case "ready":
		err = readyCommand(os.Args[2:])
	case "start":
		err = startCommand(os.Args[2:])
	case "checkpoint":
		err = checkpointCommand(os.Args[2:])
	case "finalize":
		err = finalizeCommand(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("continuity", version)
		return
	case "help", "--help", "-h":
		usage()
		return
	default:
		usage()
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "continuity:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `continuity manages a compact, repo-native PROJECT_STATE.md.

Usage:
  continuity init [--repo .] [--project name] [--objective text]
  continuity brief [--repo .]
  continuity check [--repo .] [--budget bytes]
  continuity reconcile [--repo .]
  continuity sync [--repo .]
  continuity ready [--repo .]
  continuity start --id T-001 --goal text --acceptance text --next-action text [--mode strict|soft|exploratory] [--scope path,...] [--validation command] [--repo .]
  continuity checkpoint --summary text --next-action text [--risk text] [--repo .]
  continuity finalize --summary text [--next-action text] [--repo .]

The state file is the portable shared memory. The CLI validates facts; agents and users maintain meaning.`)
}

func initCommand(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	project := flags.String("project", "", "project name")
	objective := flags.String("objective", "", "initial product objective")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := continuity.EnsureGitRepo(*repo)
	if err != nil {
		return err
	}
	path := filepath.Join(root, continuity.StateFilename)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	state := continuity.NewTemplate(path, *project, *objective, continuity.Snapshot(root), fingerprint)
	if err := state.Save(); err != nil {
		return err
	}
	report, err := continuity.InstallAdapters(root)
	if err != nil {
		return err
	}
	fmt.Println("initialized", path)
	fmt.Println(report.String())
	return nil
}

func briefCommand(args []string) error {
	root, err := parseRepo(args, "brief")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	snapshot := continuity.Snapshot(root)
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	freshness := "fresh"
	if fingerprint != state.Meta["fingerprint"] {
		freshness = "STALE"
	}
	fmt.Printf("Continuity brief — state: %s — active task: %s — branch: %s — %s\n\n", state.Meta["status"], state.Meta["active_task"], snapshot.Branch, freshness)
	if snapshot.Dirty {
		fmt.Printf("Working tree has changed files: %s\n\n", strings.Join(snapshot.ChangedFiles, ", "))
	}
	fmt.Print(state.Render())
	return nil
}

func checkCommand(args []string) error {
	flags := flag.NewFlagSet("check", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	budget := flags.Int("budget", continuity.DefaultStateBudget, "maximum state-file bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	result := state.Validate(*budget)
	snapshot := continuity.Snapshot(root)
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	if fingerprint != state.Meta["fingerprint"] {
		result.Errorf("source fingerprint is stale; run continuity reconcile, update PROJECT_STATE.md, then checkpoint or finalize")
		for _, anchor := range continuity.ReviewAnchors(root, state) {
			result.Warnf("review-required: evidence anchor %q changed since synchronization", anchor)
		}
	}
	if snapshot.Branch != "" && state.Meta["branch"] != snapshot.Branch {
		result.Warnf("state was last synchronized on branch %q; current branch is %q", state.Meta["branch"], snapshot.Branch)
	}
	if snapshot.Dirty {
		result.Warnf("working tree has uncommitted changes: %s", strings.Join(snapshot.ChangedFiles, ", "))
	}
	for _, warning := range result.Warnings {
		fmt.Println("WARN", warning)
	}
	for _, issue := range result.Errors {
		fmt.Println("ERROR", issue)
	}
	if !result.OK() {
		return fmt.Errorf("state check failed with %d error(s)", len(result.Errors))
	}
	fmt.Printf("OK %s is fresh (%d bytes, branch %s)\n", continuity.StateFilename, len(state.Render()), snapshot.Branch)
	return nil
}

func reconcileCommand(args []string) error {
	root, err := parseRepo(args, "reconcile")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	snapshot := continuity.Snapshot(root)
	lastCommit := continuity.LastStateCommit(root)
	changed, err := continuity.ChangedSinceState(root)
	if err != nil {
		return err
	}
	fmt.Printf("Recorded fingerprint: %s\nCurrent fingerprint:  %s\n", state.Meta["fingerprint"], fingerprint)
	fmt.Printf("Recorded branch: %s\nCurrent branch:  %s\n", state.Meta["branch"], snapshot.Branch)
	if lastCommit == "" {
		fmt.Println("Last state commit: none; commit the initial state after reviewing it.")
	} else {
		fmt.Println("Last state commit:", lastCommit)
	}
	if len(changed) > 0 {
		fmt.Println("Committed changes since state:")
		for _, line := range changed {
			fmt.Println(" ", line)
		}
	}
	if snapshot.Dirty {
		fmt.Println("Uncommitted changes:")
		for _, path := range snapshot.ChangedFiles {
			fmt.Println(" ", path)
		}
	}
	if fingerprint == state.Meta["fingerprint"] && !snapshot.Dirty {
		fmt.Println("No reconciliation is required.")
	} else {
		fmt.Println("Reconcile only the listed changes, update the relevant state sections, then run continuity checkpoint or finalize.")
	}
	return nil
}

func syncCommand(args []string) error {
	root, err := parseRepo(args, "sync")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	if err := synchronize(state, root); err != nil {
		return err
	}
	if err := state.Save(); err != nil {
		return err
	}
	fmt.Printf("synchronized %s on branch %s at %s\n", continuity.StateFilename, state.Meta["branch"], state.Meta["head"])
	return nil
}

func readyCommand(args []string) error {
	root, err := parseRepo(args, "ready")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	tasks, err := continuity.ReadyTasks(state.Section("Ready Queue"))
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		fmt.Println("No autonomous ready tasks.")
		return nil
	}
	for _, task := range tasks {
		fmt.Printf("%s | %s\n", task.ID, task.Title)
	}
	return nil
}

func startCommand(args []string) error {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "task ID")
	goal := flags.String("goal", "", "task goal")
	acceptance := flags.String("acceptance", "", "observable task completion criteria")
	nextAction := flags.String("next-action", "", "first concrete action")
	mode := flags.String("mode", "strict", "strict, soft, or exploratory")
	scope := flags.String("scope", "", "comma-separated repository-relative paths")
	validation := flags.String("validation", "", "preferred validation command")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" || *goal == "" || *acceptance == "" || *nextAction == "" {
		return errors.New("--id, --goal, --acceptance, and --next-action are required")
	}
	if *mode != "strict" && *mode != "soft" && *mode != "exploratory" {
		return errors.New("--mode must be strict, soft, or exploratory")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	if active := state.Meta["active_task"]; active != "" && active != "none" {
		return fmt.Errorf("task %s is already active; checkpoint or finalize it first", active)
	}
	tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		return err
	}
	byID := map[string]continuity.Task{}
	var selected continuity.Task
	found := false
	for _, task := range tasks {
		byID[task.ID] = task
		if task.ID == *id {
			selected = task
			found = true
		}
	}
	if !found {
		return fmt.Errorf("task %s is not in Ready Queue", *id)
	}
	if selected.Status != " " {
		return fmt.Errorf("task %s is not queued", *id)
	}
	for _, dependency := range selected.DependsOn {
		if task, exists := byID[dependency]; !exists || task.Status != "x" {
			return fmt.Errorf("task %s has unfinished dependency %s", *id, dependency)
		}
	}
	queue, err := continuity.UpdateTaskStatus(state.Section("Ready Queue"), *id, ">")
	if err != nil {
		return err
	}
	if err := state.ReplaceSection("Ready Queue", queue); err != nil {
		return err
	}
	active := fmt.Sprintf("Task: %s\nStatus: in_progress\nMode: %s\nGoal: %s\nAcceptance criteria: %s\nImmediate next action: %s\nEdit scope: %s\nValidation: %s", *id, *mode, *goal, *acceptance, *nextAction, emptyAsNone(*scope), emptyAsNone(*validation))
	if err := state.ReplaceSection("Active Work", active); err != nil {
		return err
	}
	state.SetMeta("active_task", *id)
	state.SetMeta("status", "working")
	if err := synchronize(state, root); err != nil {
		return err
	}
	if err := state.Save(); err != nil {
		return err
	}
	fmt.Println("started", *id)
	return nil
}

func checkpointCommand(args []string) error {
	flags := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	summary := flags.String("summary", "", "factual progress summary")
	nextAction := flags.String("next-action", "", "immediate restart action")
	risk := flags.String("risk", "none", "remaining risk or blocker")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *summary == "" || *nextAction == "" {
		return errors.New("--summary and --next-action are required")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	active := state.Meta["active_task"]
	if active == "" || active == "none" {
		return errors.New("no active task; use continuity start first")
	}
	handoff := fmt.Sprintf("Task: %s\nProgress: %s\nImmediate restart action: %s\nRisk or blocker: %s", active, *summary, *nextAction, emptyAsNone(*risk))
	if err := state.ReplaceSection("Handoff", handoff); err != nil {
		return err
	}
	activeWork := replaceField(state.Section("Active Work"), "Immediate next action", *nextAction)
	if err := state.ReplaceSection("Active Work", activeWork); err != nil {
		return err
	}
	if err := synchronize(state, root); err != nil {
		return err
	}
	if err := state.Save(); err != nil {
		return err
	}
	fmt.Println("checkpointed", active)
	return nil
}

func finalizeCommand(args []string) error {
	flags := flag.NewFlagSet("finalize", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	summary := flags.String("summary", "", "completed work summary")
	nextAction := flags.String("next-action", "Run continuity ready to inspect the next authorized task.", "next restart action")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *summary == "" {
		return errors.New("--summary is required")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	active := state.Meta["active_task"]
	if active == "" || active == "none" {
		return errors.New("no active task to finalize")
	}
	queue, err := continuity.UpdateTaskStatus(state.Section("Ready Queue"), active, "x")
	if err != nil {
		return err
	}
	if err := state.ReplaceSection("Ready Queue", queue); err != nil {
		return err
	}
	if err := state.ReplaceSection("Active Work", "Task: none\nStatus: ready\nMode: strict\nGoal: none\nAcceptance criteria: none\nImmediate next action: "+*nextAction+"\nEdit scope: none\nValidation: none"); err != nil {
		return err
	}
	recent := strings.TrimSpace(state.Section("Recent Material Changes"))
	recent = "- Completed " + active + ": " + *summary + "\n" + recent
	if err := state.ReplaceSection("Recent Material Changes", recent); err != nil {
		return err
	}
	handoff := fmt.Sprintf("Last completed: %s — %s\nImmediate restart action: %s", active, *summary, *nextAction)
	if err := state.ReplaceSection("Handoff", handoff); err != nil {
		return err
	}
	state.SetMeta("active_task", "none")
	state.SetMeta("status", "ready")
	if err := synchronize(state, root); err != nil {
		return err
	}
	if err := state.Save(); err != nil {
		return err
	}
	fmt.Println("finalized", active)
	return nil
}

func parseRepo(args []string, name string) (string, error) {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	return repoRoot(*repo)
}

func repoRoot(path string) (string, error) {
	root, err := continuity.GitRoot(path)
	if err != nil {
		return "", errors.New("run continuity init in a repository first")
	}
	return root, nil
}

func loadState(root string) (*continuity.State, error) {
	path := filepath.Join(root, continuity.StateFilename)
	state, err := continuity.LoadState(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s is missing; run continuity init", continuity.StateFilename)
	}
	return state, err
}

func synchronize(state *continuity.State, root string) error {
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	snapshot := continuity.Snapshot(root)
	head := snapshot.Head
	if head == "" {
		head = "unborn"
	}
	branch := snapshot.Branch
	if branch == "" {
		branch = "unborn"
	}
	state.SetMeta("fingerprint", fingerprint)
	state.SetMeta("branch", branch)
	state.SetMeta("head", head)
	state.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
	return nil
}

func replaceField(content, field, value string) string {
	lines := strings.Split(content, "\n")
	prefix := field + ":"
	for index, line := range lines {
		if strings.HasPrefix(line, prefix) {
			lines[index] = prefix + " " + value
			return strings.Join(lines, "\n")
		}
	}
	return strings.TrimSpace(content) + "\n" + prefix + " " + value
}

func emptyAsNone(value string) string {
	if strings.TrimSpace(value) == "" {
		return "none"
	}
	return value
}
