package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	case "compact":
		err = compactCommand(os.Args[2:])
	case "sync":
		err = syncCommand(os.Args[2:])
	case "ready":
		err = readyCommand(os.Args[2:])
	case "task":
		err = taskCommand(os.Args[2:])
	case "plan":
		err = planCommand(os.Args[2:])
	case "policy":
		err = policyCommand(os.Args[2:])
	case "fact":
		err = factCommand(os.Args[2:])
	case "blocker":
		err = blockerCommand(os.Args[2:])
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
	fmt.Fprintln(os.Stderr, `continuity manages a compact, repo-native PROJECT_STATE.md.

Usage:
  continuity init [--repo .] [--project name] [--objective text]
  continuity brief [--repo .]
  continuity check [--repo .] [--budget bytes]
  continuity reconcile [--repo .]
  continuity sync [--repo .]
  continuity compact [--repo .]
  continuity ready [--repo .]
  continuity task add --id T-001 --title text --accepts text [--approved|--derived --rationale text --outcome O-001 --scope path --validates command] [--depends none] [--autonomous yes|no] [--repo .]
  continuity plan init --intent text [--project name] [--repo .]
  continuity plan validate|status [--repo .]
  continuity plan approve --approved --milestone M-001 [--repo .]
  continuity plan activate --approved --milestone M-002 [--repo .]
  continuity plan milestone|source|constraint|assumption add ... [--repo .]
  continuity plan outcome add|complete ... [--repo .]
  continuity plan decision add|resolve ... [--repo .]
  continuity plan delta add|approve ... [--repo .]
  continuity policy add|check|status ... [--repo .]
  continuity fact add|confirm|status ... [--repo .]
  continuity blocker add|resolve|status ... [--repo .]
  continuity status [--repo .]
  continuity context [--session codex-session-id|--owner harness:id] [--scope path,...] [--limit bytes] [--repo .]
  continuity heartbeat [--id T-001] --session id [--repo .]
  continuity release [--id T-001] --session id [--repo .]
  continuity recover-lock --force [--repo .]
  continuity start --id T-001 --session id|--owner id [--goal text] [--acceptance text] [--next-action text] [--selection-reason text] [--mode strict|soft|exploratory] [--scope path,...] [--validation command] [--repo .]
  continuity checkpoint --summary text --next-action text --session id|--owner id [--id T-001] [--risk text] [--repo .]
  continuity finalize [--id T-001] --commit revision --summary text --evidence 'command: result; command: result' [--session id|--owner id] [--repo .]
  continuity integrate --id T-001 [--repo .]
  continuity hook codex session-start|subagent-start|user-prompt|pretool

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
	planPath := continuity.PlanPath(root)
	if _, err := os.Stat(planPath); err == nil {
		return fmt.Errorf("%s already exists", continuity.PlanFilename)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	plan := continuity.NewPlan(planPath, *project, *objective)
	if err := plan.Save(); err != nil {
		return err
	}
	fingerprint, err = continuity.Fingerprint(root)
	if err != nil {
		return err
	}
	state := continuity.NewTemplate(path, *project, *objective, continuity.Snapshot(root), fingerprint)
	state.SetMeta("plan", continuity.PlanFilename)
	state.SetMeta("plan_status", "draft")
	state.SetMeta("plan_active_milestone", "none")
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
	var plan *continuity.Plan
	if plan, err = loadOptionalPlan(root); err != nil {
		result.Errorf("delivery plan: %v", err)
	} else if plan != nil {
		planResult := plan.Validate()
		for _, warning := range planResult.Warnings {
			result.Warnf("delivery plan: %s", warning)
		}
		for _, issue := range planResult.Errors {
			result.Errorf("delivery plan: %s", issue)
		}
		if state.Meta["plan_status"] != "" && state.Meta["plan_status"] != plan.Meta["status"] {
			result.Errorf("delivery plan status %q does not match PROJECT_STATE.md projection %q", plan.Meta["status"], state.Meta["plan_status"])
		}
		if state.Meta["plan_active_milestone"] != "" && state.Meta["plan_active_milestone"] != plan.Meta["active_milestone"] {
			result.Errorf("delivery plan active milestone %q does not match PROJECT_STATE.md projection %q", plan.Meta["active_milestone"], state.Meta["plan_active_milestone"])
		}
	}
	if policies, policyErr := loadOptionalPolicyDocument(root); policyErr != nil {
		result.Errorf("policy checks: %v", policyErr)
	} else if policies != nil {
		for _, policy := range policies.Policies {
			if err := continuity.ValidatePolicy(policy); err != nil {
				result.Errorf("policy %s: %v", policy.ID, err)
			}
		}
	}
	if facts, factErr := state.FactCards(); factErr != nil {
		result.Errorf("fact cards: %v", factErr)
	} else {
		for _, fact := range facts {
			if fact.Status == "stale" {
				result.Warnf("stale fact %s (%s) needs a fresh evidence receipt before overlapping work", fact.ID, fact.Source)
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
	if blockers, blockerErr := state.BlockerCards(); blockerErr != nil {
		result.Errorf("blocker cards: %v", blockerErr)
	} else {
		for _, blocker := range blockers {
			if blocker.Status == "open" {
				result.Warnf("open blocker %s affects %s: %s", blocker.ID, strings.Join(blocker.Scope, ","), blocker.NextAction)
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
	tasks, taskErr := continuity.ParseTasks(state.Section("Ready Queue"))
	if taskErr == nil {
		known := map[string]continuity.Task{}
		for _, task := range tasks {
			known[task.ID] = task
			// Historic completed legacy tasks and draft-only human gates remain
			// readable during adoption. Every task that can autonomously execute
			// must be bounded by the currently approved plan outcome.
			if plan != nil && task.Outcome != "" {
				if _, err := validateTaskPlanContract(plan, task, task.Status != "x"); err != nil {
					result.Errorf("task %s plan contract: %v", task.ID, err)
				}
			} else if plan != nil && task.Autonomous && task.Status != "x" {
				result.Errorf("task %s plan contract: task does not link to an approved outcome", task.ID)
			}
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
	if facts, factErr := state.FactCards(); factErr != nil {
		return fmt.Errorf("fact cards: %w", factErr)
	} else {
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
			fmt.Println("Run continuity sync to mark affected fresh facts stale, then confirm reviewed facts before overlapping work.")
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
	aged := []continuity.FactCard{}
	if err := continuity.WithCoordinationLock(root, func() error {
		state, err := loadState(root)
		if err != nil {
			return err
		}
		aged, err = synchronizeWithAging(state, root)
		if err != nil {
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
	if len(aged) > 0 {
		ids := make([]string, 0, len(aged))
		for _, fact := range aged {
			ids = append(ids, fact.ID)
		}
		fmt.Printf("marked stale from changed evidence: %s\n", strings.Join(ids, ", "))
	}
	return nil
}

type readyCandidate struct {
	Task            continuity.Task
	Outcome         *continuity.Outcome
	OutcomePriority int
	Unblocks        int
}

// readyCandidates presents evidence for an agent's choice but deliberately
// does not claim on its behalf. The agent still records why the suggested task
// is the best next step when it calls start.
func readyCandidates(state *continuity.State, plan *continuity.Plan, claims []continuity.Claim) ([]readyCandidate, error) {
	if plan != nil && plan.Meta["status"] != "approved" {
		return nil, nil
	}
	all, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		return nil, err
	}
	if plan != nil {
		for _, task := range all {
			if task.Outcome != "" {
				if _, err := validateTaskPlanContract(plan, task, task.Status != "x"); err != nil {
					return nil, fmt.Errorf("task %s: %w", task.ID, err)
				}
			} else if task.Autonomous && task.Status != "x" {
				return nil, fmt.Errorf("task %s does not link to an approved outcome", task.ID)
			}
		}
	}
	ready, err := continuity.ReadyTasks(state.Section("Ready Queue"))
	if err != nil {
		return nil, err
	}
	claimed := map[string]bool{}
	for _, claim := range claims {
		claimed[claim.TaskID] = true
	}
	candidates := make([]readyCandidate, 0, len(ready))
	for _, task := range ready {
		if claimed[task.ID] {
			continue
		}
		candidate := readyCandidate{Task: task, Unblocks: directUnblocks(task.ID, all)}
		if plan != nil {
			outcome, err := validateTaskPlanContract(plan, task, true)
			if err != nil {
				return nil, err
			}
			candidate.Outcome = &outcome
			candidate.OutcomePriority = outcome.Priority
		}
		candidates = append(candidates, candidate)
	}
	sort.SliceStable(candidates, func(left, right int) bool {
		if candidates[left].OutcomePriority != candidates[right].OutcomePriority {
			return candidates[left].OutcomePriority < candidates[right].OutcomePriority
		}
		if candidates[left].Unblocks != candidates[right].Unblocks {
			return candidates[left].Unblocks > candidates[right].Unblocks
		}
		return candidates[left].Task.ID < candidates[right].Task.ID
	})
	return candidates, nil
}

func directUnblocks(taskID string, tasks []continuity.Task) int {
	unblocks := 0
	for _, task := range tasks {
		if task.Status == "x" {
			continue
		}
		for _, dependency := range task.DependsOn {
			if dependency == taskID {
				unblocks++
				break
			}
		}
	}
	return unblocks
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
	if err := requireUsableState(state); err != nil {
		return err
	}
	if err := requireFreshState(root, state); err != nil {
		return err
	}
	plan, err := loadOptionalPlan(root)
	if err != nil {
		return fmt.Errorf("load delivery plan: %w", err)
	}
	if plan != nil {
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("delivery plan is invalid: %s", strings.Join(result.Errors, "; "))
		}
	}
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return err
	}
	candidates, err := readyCandidates(state, plan, claims)
	if err != nil {
		return fmt.Errorf("ready task contract: %w", err)
	}
	if len(candidates) == 0 {
		if plan != nil && plan.Meta["status"] != "approved" {
			fmt.Println("No autonomous ready tasks: the delivery plan is still a draft.")
		} else {
			fmt.Println("No autonomous ready tasks.")
			for _, blocked := range blockedAutonomousTasks(state, claims) {
				fmt.Println("Blocked:", blocked)
			}
			if plan != nil && plan.Meta["status"] == "approved" {
				fmt.Println("If no task is blocked, inspect the active approved outcome and derive only the next focused technical step from an unclaimed integration checkout.")
			}
		}
		return nil
	}
	for _, candidate := range candidates {
		fmt.Printf("%s | %s", candidate.Task.ID, candidate.Task.Title)
		if candidate.Outcome != nil {
			fmt.Printf(" | outcome: %s (priority %d) | scope: %s", candidate.Outcome.ID, candidate.OutcomePriority, strings.Join(candidate.Task.Scope, ", "))
		}
		fmt.Printf(" | unblocks: %d\n", candidate.Unblocks)
	}
	fmt.Println("Order uses active-outcome priority, direct unblocks, then task ID. Before claiming, record the evidence-based selection reason with continuity start.")
	return nil
}

func blockedAutonomousTasks(state *continuity.State, claims []continuity.Claim) []string {
	tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		return nil
	}
	byID := make(map[string]continuity.Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	claimed := map[string]bool{}
	for _, claim := range claims {
		claimed[claim.TaskID] = true
	}
	blocked := []string{}
	for _, task := range tasks {
		if task.Status != " " || !task.Autonomous || claimed[task.ID] {
			continue
		}
		waiting := []string{}
		for _, dependency := range task.DependsOn {
			if prerequisite, exists := byID[dependency]; !exists || prerequisite.Status != "x" {
				waiting = append(waiting, dependency)
			}
		}
		if len(waiting) > 0 {
			blocked = append(blocked, fmt.Sprintf("%s waits for %s", task.ID, strings.Join(waiting, ", ")))
		}
	}
	return blocked
}

// taskCommand adds work only from an unclaimed integration checkout. Legacy
// repositories use an explicit user approval; approved delivery plans also
// permit a compact agent-derived technical task inside an existing outcome.
func taskCommand(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: continuity task add --id T-001 --title text --accepts text [--approved|--derived --rationale text --outcome O-001 --scope path --validates command]")
	}
	return taskAddCommand(args[1:])
}

func taskAddCommand(args []string) error {
	flags := flag.NewFlagSet("task add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "new task ID")
	title := flags.String("title", "", "user-approved task title")
	depends := flags.String("depends", "none", "comma-separated prerequisite task IDs or none")
	autonomous := flags.String("autonomous", "yes", "whether an agent may claim the task: yes or no")
	acceptance := flags.String("accepts", "", "observable completion criteria")
	approved := flags.Bool("approved", false, "confirm that a user approved this task")
	outcomeID := flags.String("outcome", "", "active approved outcome for a planned implementation task")
	scope := flags.String("scope", "", "comma-separated focused repository-relative paths")
	validation := flags.String("validates", "", "focused validation command for a planned implementation task")
	derived := flags.Bool("derived", false, "record an agent-derived technical task within the approved outcome")
	rationale := flags.String("rationale", "", "one-line reason this derived task is the best next technical step")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *autonomous != "yes" && *autonomous != "no" {
		return errors.New("--autonomous must be yes or no")
	}
	if err := requireSingleLine(map[string]string{
		"--id":      *id,
		"--title":   *title,
		"--depends": *depends,
		"--accepts": *acceptance,
	}); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"--outcome":   *outcomeID,
		"--scope":     *scope,
		"--validates": *validation,
		"--rationale": *rationale,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must be one line", name)
		}
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	var taskCommit string
	if err := continuity.WithCoordinationLock(root, func() error {
		if claim, err := continuity.FindClaimForWorktree(root); err != nil {
			return err
		} else if claim != nil {
			return fmt.Errorf("cannot change the approved task queue while this worktree is claimed by %s for %s", claim.Owner, claim.TaskID)
		}
		state, err := loadState(root)
		if err != nil {
			return err
		}
		if err := requireUsableState(state); err != nil {
			return err
		}
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(changeArchivePath(*id)))); statErr == nil {
			return fmt.Errorf("task ID %s already has a committed change archive and cannot be reused", *id)
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
		if err := requireFreshState(root, state); err != nil {
			return err
		}
		plan, err := loadOptionalPlan(root)
		if err != nil {
			return fmt.Errorf("load delivery plan: %w", err)
		}
		if plan != nil {
			if result := plan.Validate(); !result.OK() {
				return fmt.Errorf("delivery plan is invalid: %s", strings.Join(result.Errors, "; "))
			}
		}
		var task continuity.Task
		if plan == nil {
			if !*approved {
				return errors.New("--approved is required; Continuity cannot infer a new product task")
			}
			if *derived || strings.TrimSpace(*outcomeID) != "" || strings.TrimSpace(*scope) != "" || strings.TrimSpace(*validation) != "" || strings.TrimSpace(*rationale) != "" {
				return errors.New("--derived, --outcome, --scope, --validates, and --rationale require an approved PROJECT_PLAN.md")
			}
			task = continuity.Task{
				ID:         *id,
				Title:      *title,
				DependsOn:  parseTaskDependencies(*depends),
				Autonomous: *autonomous == "yes",
				Acceptance: *acceptance,
			}
		} else {
			if plan.Meta["status"] != "approved" {
				return errors.New("cannot add an implementation task while the delivery plan is a draft; approve the milestone first")
			}
			if *approved == *derived {
				return errors.New("an approved plan task needs exactly one provenance: --approved for a user-specified task or --derived for an agent-derived technical task")
			}
			if err := requireSingleLine(map[string]string{
				"--outcome":   *outcomeID,
				"--scope":     *scope,
				"--validates": *validation,
			}); err != nil {
				return err
			}
			if *derived {
				if err := requireSingleLine(map[string]string{"--rationale": *rationale}); err != nil {
					return err
				}
			} else if strings.TrimSpace(*rationale) != "" {
				return errors.New("--rationale is only valid with --derived")
			}
			taskScope, err := planPaths(*scope)
			if err != nil {
				return err
			}
			task = continuity.Task{
				ID:         *id,
				Title:      *title,
				Outcome:    *outcomeID,
				Scope:      taskScope,
				DependsOn:  parseTaskDependencies(*depends),
				Autonomous: *autonomous == "yes",
				Validation: *validation,
				Acceptance: *acceptance,
			}
			if *derived {
				task.Rationale = *rationale
			}
			if _, err := validateTaskPlanContract(plan, task, true); err != nil {
				return fmt.Errorf("task contract: %w", err)
			}
			if err := continuity.EnsureCommitIdentity(root); err != nil {
				return err
			}
		}
		queue, err := continuity.AddTask(state.Section("Ready Queue"), continuity.Task{
			ID:         task.ID,
			Title:      task.Title,
			Outcome:    task.Outcome,
			Scope:      task.Scope,
			DependsOn:  task.DependsOn,
			Autonomous: task.Autonomous,
			Validation: task.Validation,
			Acceptance: task.Acceptance,
			Rationale:  task.Rationale,
		})
		if err != nil {
			return err
		}
		if err := state.ReplaceSection("Ready Queue", queue); err != nil {
			return err
		}
		if err := synchronize(state, root); err != nil {
			return err
		}
		if err := state.Save(); err != nil {
			return err
		}
		if plan != nil {
			taskCommit, err = continuity.CommitFiles(root, "docs(tasks): queue "+task.ID, continuity.StateFilename)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if *derived {
		fmt.Printf("added derived task %s for %s and committed %s\n", *id, *outcomeID, taskCommit)
	} else if taskCommit != "" {
		fmt.Printf("added approved task %s and committed %s\n", *id, taskCommit)
	} else {
		fmt.Println("added approved task", *id)
	}
	return nil
}

func startCommand(args []string) error {
	flags := flag.NewFlagSet("start", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "task ID")
	goal := flags.String("goal", "", "task goal; defaults to the queued task title")
	acceptance := flags.String("acceptance", "", "completion criteria; defaults to the queued task contract")
	nextAction := flags.String("next-action", "", "first concrete action; defaults to reading scoped context")
	mode := flags.String("mode", "strict", "strict, soft, or exploratory")
	scope := flags.String("scope", "", "comma-separated repository-relative paths; defaults to the queued task scope")
	validation := flags.String("validation", "", "validation command; defaults to the queued task contract")
	selectionReason := flags.String("selection-reason", "", "why this ready task best advances, unblocks, or de-risks the approved outcome")
	session := flags.String("session", "", "Codex session ID from continuity context")
	owner := flags.String("owner", "", "stable worker identity")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{"--id": *id}); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"--goal":             *goal,
		"--acceptance":       *acceptance,
		"--next-action":      *nextAction,
		"--scope":            *scope,
		"--validation":       *validation,
		"--selection-reason": *selectionReason,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must be one line", name)
		}
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
	if err := requireUsableState(state); err != nil {
		return err
	}
	if err := requireFreshState(root, state); err != nil {
		return err
	}
	plan, err := loadOptionalPlan(root)
	if err != nil {
		return fmt.Errorf("load delivery plan: %w", err)
	}
	if plan != nil {
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("delivery plan is invalid: %s", strings.Join(result.Errors, "; "))
		}
		if plan.Meta["status"] != "approved" {
			return errors.New("delivery plan is still a draft; explicit plan approval is required before claiming implementation work")
		}
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
	claimScope := parseScope(*scope)
	claimGoal := strings.TrimSpace(*goal)
	if claimGoal == "" {
		claimGoal = selected.Title
	}
	claimAcceptance := strings.TrimSpace(*acceptance)
	if claimAcceptance == "" {
		claimAcceptance = selected.Acceptance
	}
	claimValidation := strings.TrimSpace(*validation)
	if claimValidation == "" {
		claimValidation = selected.Validation
	}
	claimNextAction := strings.TrimSpace(*nextAction)
	if claimNextAction == "" {
		claimNextAction = "Read scoped continuity context and begin " + selected.ID + "."
	}
	claimSelectionReason := strings.TrimSpace(*selectionReason)
	if plan != nil {
		if _, err := validateTaskPlanContract(plan, selected, true); err != nil {
			return fmt.Errorf("task contract: %w", err)
		}
		if strings.TrimSpace(*goal) != "" && claimGoal != selected.Title {
			return fmt.Errorf("task %s goal is fixed by its approved contract; omit --goal or use %q", selected.ID, selected.Title)
		}
		if strings.TrimSpace(*acceptance) != "" && claimAcceptance != selected.Acceptance {
			return fmt.Errorf("task %s acceptance is fixed by its approved contract; omit --acceptance or use %q", selected.ID, selected.Acceptance)
		}
		if len(claimScope) == 0 {
			claimScope = append([]string(nil), selected.Scope...)
		} else {
			claimScope, err = planPaths(*scope)
			if err != nil {
				return err
			}
		}
		if !pathsContainedBy(claimScope, selected.Scope) {
			return fmt.Errorf("claim scope %s exceeds task %s scope %s", strings.Join(claimScope, ", "), selected.ID, strings.Join(selected.Scope, ", "))
		}
		if claimValidation != selected.Validation {
			return fmt.Errorf("task %s validation is fixed by its approved contract; omit --validation or use %q", selected.ID, selected.Validation)
		}
		if claimSelectionReason == "" {
			return errors.New("--selection-reason is required for planned work; use continuity ready evidence to explain the choice")
		}
	} else if len(claimScope) == 0 && len(selected.Scope) > 0 {
		claimScope = append([]string(nil), selected.Scope...)
	}
	if facts, err := state.FactCards(); err != nil {
		return fmt.Errorf("fact cards: %w", err)
	} else if stale := continuity.StaleFactCardsForScopes(facts, claimScope); len(stale) > 0 {
		ids := make([]string, 0, len(stale))
		for _, fact := range stale {
			ids = append(ids, fact.ID)
		}
		return fmt.Errorf("task %s overlaps stale fact(s) %s; inspect their source paths and run continuity fact confirm --id <fact> --evidence 'review result' before claiming", selected.ID, strings.Join(ids, ", "))
	}
	if blockers, err := state.BlockerCards(); err != nil {
		return fmt.Errorf("blocker cards: %w", err)
	} else if open := continuity.OpenBlockerCardsForScopes(blockers, claimScope); len(open) > 0 {
		ids := make([]string, 0, len(open))
		for _, blocker := range open {
			ids = append(ids, blocker.ID)
		}
		return fmt.Errorf("task %s overlaps open blocker(s) %s; gather new evidence and resolve the blocker before retrying", selected.ID, strings.Join(ids, ", "))
	}
	if err := requireSingleLine(map[string]string{
		"--goal":        claimGoal,
		"--acceptance":  claimAcceptance,
		"--next-action": claimNextAction,
	}); err != nil {
		return err
	}
	claimOwner := strings.TrimSpace(*owner)
	if claimOwner == "" {
		claimOwner = ownerForSession(*session)
	}
	if claimOwner == "" {
		// A pid-derived fallback produces a claim the next process cannot
		// address, so heartbeat, checkpoint, and release all become unusable.
		return errors.New("--owner or --session is required; use an identity the next session can reproduce, for example claude:$(git branch --show-current)")
	}
	snapshot := continuity.Snapshot(root)
	if strings.TrimSpace(snapshot.Branch) == "" {
		return errors.New("writer claims require a named Git branch; create or switch to a branch before starting work")
	}
	claim, err := continuity.ClaimTask(root, continuity.ClaimRequest{
		TaskID:          *id,
		Owner:           claimOwner,
		Worktree:        root,
		Branch:          snapshot.Branch,
		BaseCommit:      snapshot.Head,
		Scope:           claimScope,
		Goal:            claimGoal,
		Acceptance:      claimAcceptance,
		Mode:            *mode,
		NextAction:      claimNextAction,
		Validation:      claimValidation,
		SelectionReason: claimSelectionReason,
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
	claim, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		return err
	}
	if claim != nil {
		if *id != "" && claim.TaskID != *id {
			return fmt.Errorf("this worktree owns %s, not requested task %s", claim.TaskID, *id)
		}
		if claimOwner != "" && claim.Owner != claimOwner {
			return fmt.Errorf("this worktree claim belongs to %s, not %s", claim.Owner, claimOwner)
		}
		if claimOwner == "" {
			claimOwner = claim.Owner
		}
	}
	policyEvidence, err := enforceFinalizationPolicies(root, claim)
	if err != nil {
		return err
	}
	completionEvidence := append(parseEvidence(*evidence), policyEvidence...)
	completion, err := continuity.CompleteClaim(root, continuity.CompletionRequest{
		TaskID:   *id,
		Owner:    claimOwner,
		Worktree: root,
		Commit:   resolvedCommit,
		Summary:  *summary,
		Evidence: completionEvidence,
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

// synchronizeWithAging refreshes the compact Git snapshot and makes the only
// permitted automatic fact change: fresh facts whose declared evidence paths
// changed become stale. It never changes a fact's meaning or marks one fresh;
// confirmation always needs explicit new review evidence.
func synchronizeWithAging(state *continuity.State, root string) ([]continuity.FactCard, error) {
	aged, err := ageFactsBeforeSynchronize(root, state)
	if err != nil {
		return nil, err
	}
	if err := synchronize(state, root); err != nil {
		return nil, err
	}
	return aged, nil
}

// requireUsableState refuses a structurally invalid state but lets a state that
// is merely over its size budget proceed with a warning. Blocking here would be
// a deadlock: the only compactor runs inside integrate, so refusing work intake
// on size alone leaves no command able to reduce the size. `continuity check`
// still reports the overflow as an error with a non-zero exit.
// runCompact exposes the compactor that otherwise only runs inside integrate.
// Without it an over-budget state cannot be reduced by any command.
func compactCommand(args []string) error {
	root, err := parseRepo(args, "compact")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	before := len(state.Render())
	if err := compactDurableState(root, state, ""); err != nil {
		return err
	}
	if err := state.Save(); err != nil {
		return err
	}
	after := len(state.Render())
	budget := continuity.DefaultStateBudget
	fmt.Printf("compacted PROJECT_STATE.md from %d to %d bytes (budget %d)\n", before, after, budget)
	if after > budget {
		fmt.Fprintf(os.Stderr, "WARN still %d bytes over budget; the compactor only trims Ready Queue, Verification and Recent Material Changes - move detail from the other sections into PROJECT_HISTORY/\n", after-budget)
	}
	return nil
}

func requireUsableState(state *continuity.State) error {
	result := state.Validate(continuity.DefaultStateBudget)
	if result.OK() {
		return nil
	}
	structural := make([]string, 0, len(result.Errors))
	overflow := ""
	for _, item := range result.Errors {
		if strings.HasPrefix(item, "state is ") && strings.Contains(item, "budget is ") {
			overflow = item
			continue
		}
		structural = append(structural, item)
	}
	if len(structural) > 0 {
		return fmt.Errorf("PROJECT_STATE.md is invalid: %s", strings.Join(structural, "; "))
	}
	fmt.Fprintf(os.Stderr, "WARN %s; run continuity compact to reduce it\n", overflow)
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
	facts, err := state.FactCards()
	if err != nil {
		return fmt.Errorf("fact cards: %w", err)
	}
	changed, err := evidenceChangedPathsSinceState(root)
	if err != nil {
		return err
	}
	fresh := []string{}
	for _, fact := range continuity.FactCardsAffectedByPaths(facts, changed) {
		if fact.Status == "fresh" {
			fresh = append(fresh, fact.ID)
		}
	}
	if len(fresh) > 0 {
		return fmt.Errorf("source-linked fact(s) %s have unsynchronized evidence changes; run continuity reconcile and continuity sync before claiming work", strings.Join(fresh, ", "))
	}
	return nil
}

func parseTaskDependencies(value string) []string {
	if value = strings.TrimSpace(value); value == "" || strings.EqualFold(value, "none") {
		return nil
	}
	dependencies := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			dependencies = append(dependencies, item)
		}
	}
	return dependencies
}
