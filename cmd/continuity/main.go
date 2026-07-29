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
	case "status":
		err = statusCommand(os.Args[2:])
	case "context":
		err = contextCommand(os.Args[2:])
	case "heartbeat":
		err = heartbeatCommand(os.Args[2:])
	case "release":
		err = releaseCommand(os.Args[2:])
	case "recover-lock":
		err = recoverLockCommand(os.Args[2:])
	case "start":
		err = startCommand(os.Args[2:])
	case "checkpoint":
		err = checkpointCommand(os.Args[2:])
	case "finalize":
		err = finalizeCommand(os.Args[2:])
	case "integrate":
		err = integrateCommand(os.Args[2:])
	case "codex-hook":
		err = codexHookCommand(os.Args[2:])
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
  continuity status [--repo .]
  continuity context [--session codex-session-id] [--limit bytes] [--repo .]
  continuity heartbeat [--id T-001] --session id [--repo .]
  continuity release [--id T-001] --session id [--repo .]
  continuity recover-lock --force [--repo .]
  continuity start --id T-001 --goal text --acceptance text --next-action text [--session id|--owner id] [--mode strict|soft|exploratory] [--scope path,...] [--validation command] [--repo .]
  continuity checkpoint [--id T-001] --summary text --next-action text [--risk text] [--session id|--owner id] [--repo .]
  continuity finalize [--id T-001] --commit revision --summary text --evidence 'command: result; command: result' [--session id|--owner id] [--repo .]
  continuity integrate --id T-001 [--repo .]
  continuity codex-hook session-start|subagent-start|pretool

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
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return err
	}
	completions, err := continuity.ListCompletions(root)
	if err != nil {
		return err
	}
	fmt.Printf("Continuity brief — state: %s — branch: %s — %s — live claims: %d — pending integrations: %d\n\n", state.Meta["status"], snapshot.Branch, freshness, len(claims), len(completions))
	if claim, err := continuity.FindClaimForWorktree(root); err != nil {
		return err
	} else if claim != nil {
		fmt.Printf("This worktree is claimed by %s for %s (scope: %s).\n\n", claim.Owner, claim.TaskID, strings.Join(claim.Scope, ", "))
	}
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
		result.Errorf("source fingerprint is stale; run continuity reconcile, update PROJECT_STATE.md, then sync")
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
	tasks, taskErr := continuity.ParseTasks(state.Section("Ready Queue"))
	if taskErr == nil {
		known := map[string]continuity.Task{}
		for _, task := range tasks {
			known[task.ID] = task
		}
		claims, err := continuity.ListClaims(root)
		if err != nil {
			return err
		}
		for _, claim := range claims {
			task, exists := known[claim.TaskID]
			if !exists {
				result.Errorf("live claim %s is not in Ready Queue", claim.TaskID)
				continue
			}
			if task.Status == "x" {
				result.Errorf("live claim %s targets an already integrated task", claim.TaskID)
			}
			if claim.Stale(time.Now().UTC()) {
				result.Warnf("live claim %s owned by %s is stale; inspect it before recovery", claim.TaskID, claim.Owner)
			}
		}
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
		fmt.Println("Reconcile only the listed changes, update the relevant durable state sections, then run continuity sync. Live worker progress belongs in a claim checkpoint.")
	}
	return nil
}

func syncCommand(args []string) error {
	root, err := parseRepo(args, "sync")
	if err != nil {
		return err
	}
	branch := ""
	head := ""
	if err := continuity.WithCoordinationLock(root, func() error {
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
		branch = state.Meta["branch"]
		head = state.Meta["head"]
		return nil
	}); err != nil {
		return err
	}
	fmt.Printf("synchronized %s on branch %s at %s\n", continuity.StateFilename, branch, head)
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
	if result := state.Validate(continuity.DefaultStateBudget); !result.OK() {
		return fmt.Errorf("PROJECT_STATE.md is invalid: %s", strings.Join(result.Errors, "; "))
	}
	if err := requireFreshState(root, state); err != nil {
		return err
	}
	tasks, err := continuity.ReadyTasks(state.Section("Ready Queue"))
	if err != nil {
		return err
	}
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return err
	}
	claimed := map[string]bool{}
	for _, claim := range claims {
		claimed[claim.TaskID] = true
	}
	ready := 0
	for _, task := range tasks {
		if claimed[task.ID] {
			continue
		}
		ready++
		fmt.Printf("%s | %s\n", task.ID, task.Title)
	}
	if ready == 0 {
		fmt.Println("No autonomous ready tasks.")
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
	session := flags.String("session", "", "Codex session ID from continuity context")
	owner := flags.String("owner", "", "stable worker identity")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{
		"--id":          *id,
		"--goal":        *goal,
		"--acceptance":  *acceptance,
		"--next-action": *nextAction,
	}); err != nil {
		return err
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
	if result := state.Validate(continuity.DefaultStateBudget); !result.OK() {
		return fmt.Errorf("PROJECT_STATE.md is invalid: %s", strings.Join(result.Errors, "; "))
	}
	if err := requireFreshState(root, state); err != nil {
		return err
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
	if !selected.Autonomous {
		return fmt.Errorf("task %s is not autonomous; record explicit user approval before claiming it", *id)
	}
	for _, dependency := range selected.DependsOn {
		if task, exists := byID[dependency]; !exists || task.Status != "x" {
			return fmt.Errorf("task %s has unfinished dependency %s", *id, dependency)
		}
	}
	claimOwner := strings.TrimSpace(*owner)
	if claimOwner == "" {
		claimOwner = ownerForSession(*session)
	}
	if claimOwner == "" {
		claimOwner = defaultOwner()
	}
	snapshot := continuity.Snapshot(root)
	claim, err := continuity.ClaimTask(root, continuity.ClaimRequest{
		TaskID:     *id,
		Owner:      claimOwner,
		Worktree:   root,
		Branch:     snapshot.Branch,
		Scope:      parseScope(*scope),
		Goal:       *goal,
		Acceptance: *acceptance,
		Mode:       *mode,
		NextAction: *nextAction,
		Validation: *validation,
	})
	if err != nil {
		return err
	}
	fmt.Printf("started %s for %s in %s\n", claim.TaskID, claim.Owner, claim.Worktree)
	return nil
}

func checkpointCommand(args []string) error {
	flags := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "active task ID; inferred from this worktree when omitted")
	summary := flags.String("summary", "", "factual progress summary")
	nextAction := flags.String("next-action", "", "immediate restart action")
	risk := flags.String("risk", "none", "remaining risk or blocker")
	session := flags.String("session", "", "Codex session ID from continuity context")
	owner := flags.String("owner", "", "stable worker identity")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{
		"--summary":     *summary,
		"--next-action": *nextAction,
		"--risk":        *risk,
	}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	claimOwner := strings.TrimSpace(*owner)
	if claimOwner == "" {
		claimOwner = ownerForSession(*session)
	}
	claim, err := continuity.CheckpointClaim(root, continuity.CheckpointRequest{
		TaskID:     *id,
		Owner:      claimOwner,
		Worktree:   root,
		Summary:    *summary,
		NextAction: *nextAction,
		Risk:       *risk,
	})
	if err != nil {
		return err
	}
	fmt.Println("checkpointed", claim.TaskID)
	return nil
}

func finalizeCommand(args []string) error {
	flags := flag.NewFlagSet("finalize", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "active task ID; inferred from this worktree when omitted")
	commit := flags.String("commit", "", "worker commit containing the completed result")
	summary := flags.String("summary", "", "completed work summary")
	evidence := flags.String("evidence", "", "semicolon-separated validation evidence")
	session := flags.String("session", "", "Codex session ID from continuity context")
	owner := flags.String("owner", "", "stable worker identity")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{
		"--commit":   *commit,
		"--summary":  *summary,
		"--evidence": *evidence,
	}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	resolvedCommit, err := continuity.ResolveCommit(root, *commit)
	if err != nil {
		return fmt.Errorf("resolve commit %s: %w", *commit, err)
	}
	claimOwner := strings.TrimSpace(*owner)
	if claimOwner == "" {
		claimOwner = ownerForSession(*session)
	}
	completion, err := continuity.CompleteClaim(root, continuity.CompletionRequest{
		TaskID:   *id,
		Owner:    claimOwner,
		Worktree: root,
		Commit:   resolvedCommit,
		Summary:  *summary,
		Evidence: parseEvidence(*evidence),
	})
	if err != nil {
		return err
	}
	fmt.Printf("finalized %s at %s; merge it, then run continuity integrate --id %s\n", completion.TaskID, completion.Commit, completion.TaskID)
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

func requireFreshState(root string, state *continuity.State) error {
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	if fingerprint != state.Meta["fingerprint"] {
		return errors.New("PROJECT_STATE.md is stale; run continuity reconcile, update the durable facts, then continuity sync before claiming work")
	}
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
