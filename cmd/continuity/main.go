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

const version = "0.2.0"

func main() {
	if len(os.Args) < 2 {
		usage()
		return
	}
	var err error
	switch os.Args[1] {
	case "init":
		err = initCommand(os.Args[2:])
	case "context":
		err = contextCommand(os.Args[2:])
	case "recall":
		err = recallCommand(os.Args[2:])
	case "check", "doctor":
		err = checkCommand(os.Args[2:])
	case "reconcile":
		err = reconcileCommand(os.Args[2:])
	case "sync":
		err = syncCommand(os.Args[2:])
	case "brief":
		err = briefCommand(os.Args[2:])
	case "fact":
		err = factCommand(os.Args[2:])
	case "hook":
		err = hookCommand(os.Args[2:])
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
	if errors.Is(err, flag.ErrHelp) {
		// A subcommand's -h is a successful request for help, not a failure.
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "continuity:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `continuity keeps project memory that every coding-agent session can read.

Usage:
  continuity init [--repo .] [--project name] [--objective text]
  continuity context [--scope path,...] [--limit bytes] [--repo .]
  continuity recall <words> [--limit 8] [--repo .]
  continuity recall --path file[,file...] [--repo .]
  continuity check [--repo .] [--budget bytes]
  continuity reconcile [--repo .]
  continuity sync [--repo .]
  continuity brief [--repo .]
  continuity fact add|confirm|status ... [--repo .]
  continuity hook claude|codex session-start|posttool

PROJECT_PLAN.md says what to build next, docs/adr records decisions, and
PROJECT_STATE.md indexes the code, facts, pitfalls, and the last handoff.`)
}

func initCommand(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	project := flags.String("project", "", "project name")
	objective := flags.String("objective", "", "why the product exists, for the plan's Intent")
	if err := flags.Parse(args); err != nil {
		return err
	}
	root, err := continuity.EnsureGitRepo(*repo)
	if err != nil {
		return err
	}
	statePath := filepath.Join(root, continuity.StateFilename)
	planPath := continuity.PlanPath(root)
	for _, path := range []string{statePath, planPath} {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	name := *project
	if name == "" {
		name = filepath.Base(root)
	}
	if err := os.WriteFile(planPath, []byte(continuity.NewPlanText(name, *objective)), 0o644); err != nil {
		return err
	}
	report, err := continuity.InstallAdapters(root)
	if err != nil {
		return err
	}
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	if err := continuity.NewTemplate(statePath, name, continuity.Snapshot(root), fingerprint).Save(); err != nil {
		return err
	}
	fmt.Println("initialized", statePath, "and", planPath)
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
	fmt.Printf("Continuity brief — branch: %s — index %s\n\n", snapshot.Branch, freshness)
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
	if plan, err := loadPlan(root); err != nil {
		result.Errorf("%v", err)
	} else {
		planResult := plan.Validate()
		for _, warning := range planResult.Warnings {
			result.Warnf("%s: %s", continuity.PlanFilename, warning)
		}
		for _, issue := range planResult.Errors {
			result.Errorf("%s: %s", continuity.PlanFilename, issue)
		}
	}
	if _, problems, err := continuity.LoadDecisions(root); err != nil {
		result.Errorf("decision records: %v", err)
	} else {
		for _, problem := range problems {
			result.Errorf("decision record %s", problem)
		}
	}
	if facts, factErr := state.FactCards(); factErr != nil {
		result.Errorf("fact cards: %v", factErr)
	} else {
		for _, fact := range facts {
			if fact.Status == "stale" {
				result.Warnf("stale fact %s (%s) needs a fresh review receipt before it is relied on", fact.ID, fact.Source)
			}
		}
		if changed, changedErr := evidenceChangedPathsSinceState(root); changedErr != nil {
			result.Errorf("fact-card evidence changes: %v", changedErr)
		} else {
			for _, fact := range continuity.FactCardsAffectedByPaths(facts, changed) {
				if fact.Status == "fresh" {
					result.Errorf("source-linked fact %s has unsynchronized evidence changes; run continuity reconcile and continuity sync", fact.ID)
				}
			}
		}
	}
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
	facts, err := state.FactCards()
	if err != nil {
		return fmt.Errorf("fact cards: %w", err)
	}
	paths, err := continuity.ChangedPathsSinceState(root)
	if err != nil {
		return err
	}
	paths = append(paths, snapshot.ChangedFiles...)
	if affected := continuity.FactCardsAffectedByPaths(facts, paths); len(affected) > 0 {
		fmt.Println("Fact cards affected by listed changes:")
		for _, fact := range affected {
			fmt.Printf(" - %s [%s] | source: %s | paths: %s\n", fact.ID, fact.Status, fact.Source, strings.Join(fact.Paths, ","))
		}
		fmt.Println("Run continuity sync to mark affected fresh facts stale, then confirm each fact after reviewing it.")
	}
	if fingerprint == state.Meta["fingerprint"] && !snapshot.Dirty {
		fmt.Println("No reconciliation is required.")
	} else {
		fmt.Println("Reconcile only the listed changes: update the Code Map, capsules, pitfalls, and facts they affect, record new decisions in docs/adr, then run continuity sync.")
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
	aged, err := ageFactsBeforeSynchronize(root, state)
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
	if len(aged) > 0 {
		ids := make([]string, 0, len(aged))
		for _, fact := range aged {
			ids = append(ids, fact.ID)
		}
		fmt.Printf("marked stale from changed evidence: %s\n", strings.Join(ids, ", "))
	}
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
	state, err := continuity.LoadState(filepath.Join(root, continuity.StateFilename))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s is missing; run continuity init", continuity.StateFilename)
	}
	return state, err
}

func loadPlan(root string) (*continuity.Plan, error) {
	plan, err := continuity.LoadPlan(continuity.PlanPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s is missing; run continuity init or create it with Intent, Milestones, and Outcomes", continuity.PlanFilename)
	}
	return plan, err
}

// synchronize records the Git facts the index was last checked against.
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

// requireFreshState refuses a durable write while the index lags the code, so
// a new fact cannot hide changes nobody reconciled.
func requireFreshState(root string, state *continuity.State) error {
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	if fingerprint != state.Meta["fingerprint"] {
		return errors.New("PROJECT_STATE.md is stale; run continuity reconcile, update the index, then continuity sync")
	}
	facts, err := state.FactCards()
	if err != nil {
		return fmt.Errorf("fact cards: %w", err)
	}
	changed, err := evidenceChangedPathsSinceState(root)
	if err != nil {
		return err
	}
	for _, fact := range continuity.FactCardsAffectedByPaths(facts, changed) {
		if fact.Status == "fresh" {
			return fmt.Errorf("source-linked fact %s has unsynchronized evidence changes; run continuity reconcile and continuity sync first", fact.ID)
		}
	}
	return nil
}

func requireSingleLine(fields map[string]string) error {
	for name, value := range fields {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must be one line", name)
		}
	}
	return nil
}

func singleLineSnippet(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit < 4 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
