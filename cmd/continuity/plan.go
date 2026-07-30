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

// planCommand owns the durable, user-approved delivery plan. It is purposely
// separate from worker claims: drafting and approving product direction happen
// in an unclaimed integration checkout, while implementation stays claim-based.
func planCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: continuity plan init|validate|status|approve|activate|milestone|outcome|source|constraint|assumption|decision|delta")
	}
	switch args[0] {
	case "init":
		return planInitCommand(args[1:])
	case "validate":
		return planValidateCommand(args[1:])
	case "status":
		return planStatusCommand(args[1:])
	case "approve":
		return planApproveCommand(args[1:])
	case "activate":
		return planActivateCommand(args[1:])
	case "milestone":
		return planMilestoneCommand(args[1:])
	case "outcome":
		return planOutcomeCommand(args[1:])
	case "source":
		return planSourceCommand(args[1:])
	case "constraint":
		return planConstraintCommand(args[1:])
	case "assumption":
		return planAssumptionCommand(args[1:])
	case "decision":
		return planDecisionCommand(args[1:])
	case "delta":
		return planDeltaCommand(args[1:])
	default:
		return fmt.Errorf("unknown plan command %q", args[0])
	}
}

func planInitCommand(args []string) error {
	flags := flag.NewFlagSet("plan init", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	project := flags.String("project", "", "project name")
	intent := flags.String("intent", "", "one-line initial product intent")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{"--intent": *intent}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	if err := continuity.WithCoordinationLock(root, func() error {
		if err := requireUnclaimedWorktree(root); err != nil {
			return err
		}
		if err := requirePlanningStateFresh(root); err != nil {
			return err
		}
		path := continuity.PlanPath(root)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists", continuity.PlanFilename)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		plan := continuity.NewPlan(path, *project, *intent)
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("new plan is invalid: %s", strings.Join(result.Errors, "; "))
		}
		if err := plan.Save(); err != nil {
			return err
		}
		return synchronizePlanIndex(root, plan)
	}); err != nil {
		return err
	}
	fmt.Println("initialized draft", continuity.PlanFilename)
	return nil
}

func planValidateCommand(args []string) error {
	root, err := parseRepo(args, "plan validate")
	if err != nil {
		return err
	}
	plan, err := loadPlan(root)
	if err != nil {
		return err
	}
	result := plan.Validate()
	for _, warning := range result.Warnings {
		fmt.Println("WARN", warning)
	}
	for _, issue := range result.Errors {
		fmt.Println("ERROR", issue)
	}
	if !result.OK() {
		return fmt.Errorf("plan validation failed with %d error(s)", len(result.Errors))
	}
	fmt.Printf("OK %s is %s (%d bytes)\n", continuity.PlanFilename, plan.Meta["status"], len(plan.Render()))
	return nil
}

func planStatusCommand(args []string) error {
	root, err := parseRepo(args, "plan status")
	if err != nil {
		return err
	}
	plan, err := loadPlan(root)
	if err != nil {
		return err
	}
	if result := plan.Validate(); !result.OK() {
		return fmt.Errorf("%s is invalid: %s", continuity.PlanFilename, strings.Join(result.Errors, "; "))
	}
	fmt.Printf("Plan status: %s\n", plan.Meta["status"])
	fmt.Printf("Active milestone: %s\n", plan.Meta["active_milestone"])
	for _, milestone := range mustMilestones(plan) {
		fmt.Printf("- [%s] %s | %s\n", milestone.Status, milestone.ID, milestone.Title)
	}
	for _, decision := range mustDecisions(plan) {
		if !decision.Resolved {
			fmt.Printf("Open decision: %s | %s | recommendation: %s\n", decision.ID, decision.Question, decision.Recommendation)
		}
	}
	return nil
}

func planApproveCommand(args []string) error {
	flags := flag.NewFlagSet("plan approve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	milestoneID := flags.String("milestone", "", "first approved executable milestone")
	approved := flags.Bool("approved", false, "confirm explicit user approval")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*approved {
		return errors.New("--approved is required; Continuity cannot infer plan approval")
	}
	if err := requireSingleLine(map[string]string{"--milestone": *milestoneID}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	var commit string
	if err := continuity.WithCoordinationLock(root, func() error {
		if err := requireUnclaimedWorktree(root); err != nil {
			return err
		}
		if err := requirePlanningStateFresh(root); err != nil {
			return err
		}
		if err := continuity.EnsureCommitIdentity(root); err != nil {
			return err
		}
		plan, err := loadPlan(root)
		if err != nil {
			return err
		}
		if plan.Meta["status"] != "draft" {
			return fmt.Errorf("plan is already %s; use plan activate or plan delta", plan.Meta["status"])
		}
		if err := activatePlanMilestone(plan, *milestoneID, true); err != nil {
			return err
		}
		plan.SetMeta("status", "approved")
		plan.SetMeta("approved_at", time.Now().UTC().Format(time.RFC3339))
		plan.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("cannot approve invalid plan: %s", strings.Join(result.Errors, "; "))
		}
		if err := ensureNoBlockingDecision(plan, *milestoneID); err != nil {
			return err
		}
		if err := plan.Save(); err != nil {
			return err
		}
		if err := synchronizePlanIndex(root, plan); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(plan): approve "+*milestoneID, continuity.PlanFilename, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("approved %s and committed %s\n", *milestoneID, commit)
	return nil
}

func planActivateCommand(args []string) error {
	flags := flag.NewFlagSet("plan activate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	milestoneID := flags.String("milestone", "", "next approved executable milestone")
	approved := flags.Bool("approved", false, "confirm explicit user approval")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*approved {
		return errors.New("--approved is required; Continuity cannot infer milestone approval")
	}
	if err := requireSingleLine(map[string]string{"--milestone": *milestoneID}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	var commit string
	if err := continuity.WithCoordinationLock(root, func() error {
		if err := requireUnclaimedWorktree(root); err != nil {
			return err
		}
		if err := requirePlanningStateFresh(root); err != nil {
			return err
		}
		if err := continuity.EnsureCommitIdentity(root); err != nil {
			return err
		}
		plan, err := loadPlan(root)
		if err != nil {
			return err
		}
		if plan.Meta["status"] != "approved" {
			return errors.New("plan must be approved before another milestone can activate")
		}
		current := plan.Meta["active_milestone"]
		if current == "none" {
			return errors.New("approved plan has no active milestone")
		}
		if err := requireMilestoneComplete(plan, current); err != nil {
			return err
		}
		if err := activatePlanMilestone(plan, *milestoneID, false); err != nil {
			return err
		}
		plan.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
		if err := ensureNoBlockingDecision(plan, *milestoneID); err != nil {
			return err
		}
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("cannot activate invalid plan: %s", strings.Join(result.Errors, "; "))
		}
		if err := plan.Save(); err != nil {
			return err
		}
		if err := synchronizePlanIndex(root, plan); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(plan): activate "+*milestoneID, continuity.PlanFilename, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("activated %s and committed %s\n", *milestoneID, commit)
	return nil
}

func planMilestoneCommand(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: continuity plan milestone add --id M-001 --priority 1 --title text --accepts text")
	}
	flags := flag.NewFlagSet("plan milestone add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "milestone ID")
	priority := flags.Int("priority", 0, "lower number means earlier roadmap priority")
	title := flags.String("title", "", "milestone title")
	acceptance := flags.String("accepts", "", "observable milestone completion")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *priority <= 0 {
		return errors.New("--priority must be positive")
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--title": *title, "--accepts": *acceptance}); err != nil {
		return err
	}
	return mutateDraftPlan(*repo, func(plan *continuity.Plan) error {
		if !strings.HasPrefix(*id, "M-") {
			return errors.New("--id must begin with M-")
		}
		line := continuity.RenderMilestone(continuity.Milestone{Status: " ", ID: *id, Priority: *priority, Title: *title, Acceptance: *acceptance})
		return appendPlanSection(plan, "Milestones", line)
	})
}

func planOutcomeCommand(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: continuity plan outcome add --id O-001 --milestone M-001 --priority 1 --title text --scope path --accepts text")
	}
	flags := flag.NewFlagSet("plan outcome add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "outcome ID")
	milestone := flags.String("milestone", "", "parent milestone ID")
	priority := flags.Int("priority", 0, "lower number means earlier outcome priority")
	title := flags.String("title", "", "user-visible outcome title")
	scope := flags.String("scope", "", "comma-separated repository-relative scope")
	acceptance := flags.String("accepts", "", "observable outcome completion")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if *priority <= 0 {
		return errors.New("--priority must be positive")
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--milestone": *milestone, "--title": *title, "--scope": *scope, "--accepts": *acceptance}); err != nil {
		return err
	}
	paths, err := planPaths(*scope)
	if err != nil {
		return err
	}
	return mutateDraftPlan(*repo, func(plan *continuity.Plan) error {
		if !strings.HasPrefix(*id, "O-") || !strings.HasPrefix(*milestone, "M-") {
			return errors.New("outcome IDs must begin with O- and milestones with M-")
		}
		if !planHasMilestone(plan, *milestone) {
			return fmt.Errorf("unknown milestone %s", *milestone)
		}
		line := continuity.RenderOutcome(continuity.Outcome{Status: " ", ID: *id, Milestone: *milestone, Priority: *priority, Title: *title, Scope: paths, Acceptance: *acceptance})
		return appendPlanSection(plan, "Outcomes", line)
	})
}

func planSourceCommand(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: continuity plan source add --id S-001 --role product --ref path --confidence high --summary text")
	}
	flags := flag.NewFlagSet("plan source add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "source ID")
	role := flags.String("role", "", "product, architecture, behavior, or reference")
	ref := flags.String("ref", "", "source path or compact source reference")
	confidence := flags.String("confidence", "", "high, medium, or low")
	summary := flags.String("summary", "", "factual inferred source summary")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--role": *role, "--ref": *ref, "--confidence": *confidence, "--summary": *summary}); err != nil {
		return err
	}
	return mutateDraftPlan(*repo, func(plan *continuity.Plan) error {
		if !strings.HasPrefix(*id, "S-") {
			return errors.New("--id must begin with S-")
		}
		line := continuity.RenderSource(continuity.PlanSource{ID: *id, Role: *role, Authority: "inferred", Confidence: *confidence, Reference: *ref, Summary: *summary})
		return appendPlanSection(plan, "Sources", line)
	})
}

func planConstraintCommand(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: continuity plan constraint add --id C-001 --scope path --rule text")
	}
	flags := flag.NewFlagSet("plan constraint add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "constraint ID")
	scope := flags.String("scope", "", "comma-separated repository-relative scope")
	rule := flags.String("rule", "", "approved constraint")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--scope": *scope, "--rule": *rule}); err != nil {
		return err
	}
	paths, err := planPaths(*scope)
	if err != nil {
		return err
	}
	return mutateDraftPlan(*repo, func(plan *continuity.Plan) error {
		if !strings.HasPrefix(*id, "C-") {
			return errors.New("--id must begin with C-")
		}
		return appendPlanSection(plan, "Constraints", continuity.RenderConstraint(continuity.PlanConstraint{ID: *id, Scope: paths, Rule: *rule}))
	})
}

func planAssumptionCommand(args []string) error {
	if len(args) == 0 || args[0] != "add" {
		return errors.New("usage: continuity plan assumption add --id A-001 --impact low --statement text")
	}
	flags := flag.NewFlagSet("plan assumption add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "assumption ID")
	impact := flags.String("impact", "", "low, medium, or high")
	statement := flags.String("statement", "", "explicit low-confidence assumption")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--impact": *impact, "--statement": *statement}); err != nil {
		return err
	}
	return mutateDraftPlan(*repo, func(plan *continuity.Plan) error {
		if !strings.HasPrefix(*id, "A-") {
			return errors.New("--id must begin with A-")
		}
		return appendPlanSection(plan, "Assumptions", continuity.RenderAssumption(continuity.Assumption{ID: *id, Impact: *impact, Statement: *statement}))
	})
}

func planDecisionCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: continuity plan decision add|resolve ...")
	}
	switch args[0] {
	case "add":
		return planDecisionAddCommand(args[1:])
	case "resolve":
		return planDecisionResolveCommand(args[1:])
	default:
		return errors.New("usage: continuity plan decision add|resolve ...")
	}
}

func planDecisionAddCommand(args []string) error {
	flags := flag.NewFlagSet("plan decision add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "decision ID")
	impact := flags.String("impact", "", "low, medium, or high")
	outcome := flags.String("outcome", "none", "affected outcome ID or none")
	question := flags.String("question", "", "decision needed from the user")
	recommendation := flags.String("recommendation", "", "agent recommendation")
	evidence := flags.String("evidence", "", "compact supporting evidence")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--impact": *impact, "--outcome": *outcome, "--question": *question, "--recommendation": *recommendation, "--evidence": *evidence}); err != nil {
		return err
	}
	err := mutateDraftPlan(*repo, func(plan *continuity.Plan) error {
		if !strings.HasPrefix(*id, "D-") {
			return errors.New("--id must begin with D-")
		}
		if *outcome != "none" && !planHasOutcome(plan, *outcome) {
			return fmt.Errorf("unknown outcome %s", *outcome)
		}
		item := continuity.DecisionCard{ID: *id, Impact: *impact, Outcome: *outcome, Question: *question, Recommendation: *recommendation, Evidence: *evidence, Resolution: "none"}
		return appendPlanSection(plan, "Decisions", continuity.RenderDecision(item))
	})
	if err == nil {
		fmt.Fprintf(os.Stderr, "\aCONTINUITY DECISION REQUIRED: %s — %s\n", *id, *question)
	}
	return err
}

func planDecisionResolveCommand(args []string) error {
	flags := flag.NewFlagSet("plan decision resolve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "decision ID")
	resolution := flags.String("resolution", "", "user-approved decision resolution")
	approved := flags.Bool("approved", false, "confirm explicit user decision")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*approved {
		return errors.New("--approved is required; Continuity cannot infer a decision resolution")
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--resolution": *resolution}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	var commit string
	if err := continuity.WithCoordinationLock(root, func() error {
		if err := requireUnclaimedWorktree(root); err != nil {
			return err
		}
		if err := requirePlanningStateFresh(root); err != nil {
			return err
		}
		if err := continuity.EnsureCommitIdentity(root); err != nil {
			return err
		}
		plan, err := loadPlan(root)
		if err != nil {
			return err
		}
		item, err := planDecisionByID(plan, *id)
		if err != nil {
			return err
		}
		if item.Resolved {
			return fmt.Errorf("decision %s is already resolved", *id)
		}
		item.Resolved = true
		item.Resolution = *resolution
		updated, err := continuity.ReplacePlanRecord(plan.Section("Decisions"), *id, continuity.RenderDecision(item))
		if err != nil {
			return err
		}
		if err := plan.ReplaceSection("Decisions", updated); err != nil {
			return err
		}
		plan.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("decision resolution makes plan invalid: %s", strings.Join(result.Errors, "; "))
		}
		if err := plan.Save(); err != nil {
			return err
		}
		if err := synchronizePlanIndex(root, plan); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(plan): resolve "+*id, continuity.PlanFilename, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("resolved %s and committed %s\n", *id, commit)
	return nil
}

func planDeltaCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: continuity plan delta add|approve ...")
	}
	switch args[0] {
	case "add":
		return planDeltaAddCommand(args[1:])
	case "approve":
		return planDeltaApproveCommand(args[1:])
	default:
		return errors.New("usage: continuity plan delta add|approve ...")
	}
}

func planDeltaAddCommand(args []string) error {
	flags := flag.NewFlagSet("plan delta add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "plan delta ID")
	impact := flags.String("impact", "", "technical, product, priority, or constraint")
	summary := flags.String("summary", "", "compact proposed plan change")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requirePlanFields(map[string]string{"--id": *id, "--impact": *impact, "--summary": *summary}); err != nil {
		return err
	}
	return mutatePlan(*repo, func(plan *continuity.Plan) error {
		if plan.Meta["status"] != "approved" {
			return errors.New("plan deltas require an approved plan")
		}
		if !strings.HasPrefix(*id, "DLT-") {
			return errors.New("--id must begin with DLT-")
		}
		return appendPlanSection(plan, "Deltas", continuity.RenderDelta(continuity.PlanDelta{ID: *id, Impact: *impact, Summary: *summary}))
	})
}

func planDeltaApproveCommand(args []string) error {
	flags := flag.NewFlagSet("plan delta approve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "plan delta ID")
	approved := flags.Bool("approved", false, "confirm explicit user approval")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*approved {
		return errors.New("--approved is required; Continuity cannot infer plan-delta approval")
	}
	if err := requirePlanFields(map[string]string{"--id": *id}); err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	var commit string
	if err := continuity.WithCoordinationLock(root, func() error {
		if err := requireUnclaimedWorktree(root); err != nil {
			return err
		}
		if err := requirePlanningStateFresh(root); err != nil {
			return err
		}
		if err := continuity.EnsureCommitIdentity(root); err != nil {
			return err
		}
		plan, err := loadPlan(root)
		if err != nil {
			return err
		}
		item, err := planDeltaByID(plan, *id)
		if err != nil {
			return err
		}
		if item.Approved {
			return fmt.Errorf("plan delta %s is already approved", *id)
		}
		item.Approved = true
		updated, err := continuity.ReplacePlanRecord(plan.Section("Deltas"), *id, continuity.RenderDelta(item))
		if err != nil {
			return err
		}
		if err := plan.ReplaceSection("Deltas", updated); err != nil {
			return err
		}
		plan.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("cannot approve invalid plan delta: %s", strings.Join(result.Errors, "; "))
		}
		if err := plan.Save(); err != nil {
			return err
		}
		if err := synchronizePlanIndex(root, plan); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(plan): approve "+*id, continuity.PlanFilename, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("approved %s and committed %s\n", *id, commit)
	return nil
}

func mutateDraftPlan(repo string, mutate func(*continuity.Plan) error) error {
	return mutatePlanWithRule(repo, func(plan *continuity.Plan) error {
		if plan.Meta["status"] != "draft" {
			return errors.New("approved plans are changed through an explicit plan delta")
		}
		return mutate(plan)
	})
}

func mutatePlan(repo string, mutate func(*continuity.Plan) error) error {
	return mutatePlanWithRule(repo, mutate)
}

func mutatePlanWithRule(repo string, mutate func(*continuity.Plan) error) error {
	root, err := repoRoot(repo)
	if err != nil {
		return err
	}
	if err := continuity.WithCoordinationLock(root, func() error {
		if err := requireUnclaimedWorktree(root); err != nil {
			return err
		}
		if err := requirePlanningStateFresh(root); err != nil {
			return err
		}
		plan, err := loadPlan(root)
		if err != nil {
			return err
		}
		if err := mutate(plan); err != nil {
			return err
		}
		plan.SetMeta("updated_at", time.Now().UTC().Format(time.RFC3339))
		if result := plan.Validate(); !result.OK() {
			return fmt.Errorf("plan update is invalid: %s", strings.Join(result.Errors, "; "))
		}
		if err := plan.Save(); err != nil {
			return err
		}
		return synchronizePlanIndex(root, plan)
	}); err != nil {
		return err
	}
	fmt.Println("updated", continuity.PlanFilename)
	return nil
}

func requireUnclaimedWorktree(root string) error {
	claim, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		return err
	}
	if claim != nil {
		return fmt.Errorf("cannot change the delivery plan while this worktree is claimed by %s for %s", claim.Owner, claim.TaskID)
	}
	return nil
}

func synchronizePlanIndex(root string, plan *continuity.Plan) error {
	state, err := loadState(root)
	if err != nil {
		return err
	}
	state.SetMeta("plan", continuity.PlanFilename)
	state.SetMeta("plan_status", plan.Meta["status"])
	state.SetMeta("plan_active_milestone", plan.Meta["active_milestone"])
	if err := state.ReplaceSection("Product Intent", strings.TrimSpace(plan.Section("Intent"))); err != nil {
		return err
	}
	if err := state.ReplaceSection("Approved Outcomes", renderPlanProjection(plan)); err != nil {
		return err
	}
	if err := state.ReplaceSection("Handoff", renderPlanHandoff(plan)); err != nil {
		return err
	}
	if err := synchronize(state, root); err != nil {
		return err
	}
	return state.Save()
}

func requirePlanningStateFresh(root string) error {
	state, err := loadState(root)
	if err != nil {
		return err
	}
	return requireFreshState(root, state)
}

func renderPlanProjection(plan *continuity.Plan) string {
	lines := []string{"- Plan: `" + continuity.PlanFilename + "` (" + plan.Meta["status"] + ")"}
	active := plan.Meta["active_milestone"]
	if active == "none" {
		return strings.Join(append(lines, "- No executable milestone is approved yet."), "\n")
	}
	lines = append(lines, "- Active milestone: "+active)
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone == active && outcome.Status != "x" {
			lines = append(lines, "- "+outcome.ID+" — "+outcome.Title+" | accepts: "+outcome.Acceptance)
		}
	}
	return strings.Join(lines, "\n")
}

func renderPlanHandoff(plan *continuity.Plan) string {
	if plan.Meta["status"] != "approved" {
		return "Last completed: plan draft created.\nImmediate restart action: Continue the planning interview, validate PROJECT_PLAN.md, and wait for explicit user approval before claiming implementation work."
	}
	return "Last completed: plan approval recorded.\nImmediate restart action: Run continuity status and continuity ready; claim only a task linked to an active approved outcome."
}

func activatePlanMilestone(plan *continuity.Plan, id string, initial bool) error {
	milestones, err := plan.Milestones()
	if err != nil {
		return err
	}
	found := false
	for _, milestone := range milestones {
		if milestone.ID == id {
			found = true
			if milestone.Status == "x" {
				return fmt.Errorf("milestone %s is already complete", id)
			}
			milestone.Status = ">"
			updated, err := continuity.ReplacePlanRecord(plan.Section("Milestones"), id, continuity.RenderMilestone(milestone))
			if err != nil {
				return err
			}
			if err := plan.ReplaceSection("Milestones", updated); err != nil {
				return err
			}
			break
		}
	}
	if !found {
		return fmt.Errorf("unknown milestone %s", id)
	}
	if !initial {
		current := plan.Meta["active_milestone"]
		items, err := plan.Milestones()
		if err != nil {
			return err
		}
		for _, milestone := range items {
			if milestone.ID != current {
				continue
			}
			milestone.Status = "x"
			updated, err := continuity.ReplacePlanRecord(plan.Section("Milestones"), current, continuity.RenderMilestone(milestone))
			if err != nil {
				return err
			}
			if err := plan.ReplaceSection("Milestones", updated); err != nil {
				return err
			}
			break
		}
	}
	plan.SetMeta("active_milestone", id)
	return nil
}

func requireMilestoneComplete(plan *continuity.Plan, id string) error {
	matched := false
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone != id {
			continue
		}
		matched = true
		if outcome.Status != "x" {
			return fmt.Errorf("milestone %s still has unfinished outcome %s", id, outcome.ID)
		}
	}
	if !matched {
		return fmt.Errorf("milestone %s has no outcomes", id)
	}
	return nil
}

func ensureNoBlockingDecision(plan *continuity.Plan, milestoneID string) error {
	activeOutcomes := map[string]bool{}
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone == milestoneID {
			activeOutcomes[outcome.ID] = true
		}
	}
	if len(activeOutcomes) == 0 {
		return fmt.Errorf("milestone %s has no outcomes", milestoneID)
	}
	for _, decision := range mustDecisions(plan) {
		if !decision.Resolved && decision.Impact == "high" && (decision.Outcome == "none" || activeOutcomes[decision.Outcome]) {
			return fmt.Errorf("high-impact decision %s must be resolved before %s can execute", decision.ID, milestoneID)
		}
	}
	return nil
}

func appendPlanSection(plan *continuity.Plan, section, line string) error {
	return plan.ReplaceSection(section, continuity.AppendPlanRecord(plan.Section(section), line))
}

func loadPlan(root string) (*continuity.Plan, error) {
	plan, err := continuity.LoadPlan(continuity.PlanPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s is missing; run continuity plan init", continuity.PlanFilename)
	}
	return plan, err
}

func loadOptionalPlan(root string) (*continuity.Plan, error) {
	plan, err := continuity.LoadPlan(continuity.PlanPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return plan, err
}

func renderPlanContext(plan *continuity.Plan, scopes []string) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Delivery plan: %s | active milestone: %s\n", plan.Meta["status"], plan.Meta["active_milestone"])
	active := plan.Meta["active_milestone"]
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone != active || outcome.Status == "x" || !planScopeMatches(outcome.Scope, scopes) {
			continue
		}
		fmt.Fprintf(&builder, "Active approved outcome: %s — %s | accepts: %s\n", outcome.ID, outcome.Title, outcome.Acceptance)
	}
	for _, decision := range mustDecisions(plan) {
		if decision.Resolved || (decision.Outcome != "none" && !planOutcomeMatchesScope(plan, decision.Outcome, scopes)) {
			continue
		}
		fmt.Fprintf(&builder, "Open decision: %s | %s | recommendation: %s\n", decision.ID, decision.Question, decision.Recommendation)
	}
	return builder.String()
}

func planOutcomeMatchesScope(plan *continuity.Plan, outcomeID string, scopes []string) bool {
	for _, outcome := range mustOutcomes(plan) {
		if outcome.ID == outcomeID {
			return planScopeMatches(outcome.Scope, scopes)
		}
	}
	return false
}

func planScopeMatches(paths, scopes []string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, path := range paths {
		for _, scope := range scopes {
			path = filepath.ToSlash(filepath.Clean(path))
			scope = filepath.ToSlash(filepath.Clean(scope))
			if path == "." || scope == "." || path == scope || strings.HasPrefix(path, scope+"/") || strings.HasPrefix(scope, path+"/") {
				return true
			}
		}
	}
	return false
}

func planPaths(value string) ([]string, error) {
	paths := []string{}
	seen := map[string]bool{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" || filepath.IsAbs(item) {
			return nil, fmt.Errorf("invalid plan scope %q", item)
		}
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("plan scope %q escapes the repository", item)
		}
		if !seen[clean] {
			seen[clean] = true
			paths = append(paths, clean)
		}
	}
	if len(paths) == 0 {
		return nil, errors.New("--scope must name at least one repository-relative path")
	}
	return paths, nil
}

func requirePlanFields(fields map[string]string) error {
	if err := requireSingleLine(fields); err != nil {
		return err
	}
	for name, value := range fields {
		if strings.Contains(value, "|") {
			return fmt.Errorf("%s cannot contain |", name)
		}
	}
	return nil
}

func planHasMilestone(plan *continuity.Plan, id string) bool {
	for _, item := range mustMilestones(plan) {
		if item.ID == id {
			return true
		}
	}
	return false
}

func planHasOutcome(plan *continuity.Plan, id string) bool {
	for _, item := range mustOutcomes(plan) {
		if item.ID == id {
			return true
		}
	}
	return false
}

func planDecisionByID(plan *continuity.Plan, id string) (continuity.DecisionCard, error) {
	for _, item := range mustDecisions(plan) {
		if item.ID == id {
			return item, nil
		}
	}
	return continuity.DecisionCard{}, fmt.Errorf("unknown decision %s", id)
}

func planDeltaByID(plan *continuity.Plan, id string) (continuity.PlanDelta, error) {
	items, err := plan.Deltas()
	if err != nil {
		return continuity.PlanDelta{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return continuity.PlanDelta{}, fmt.Errorf("unknown plan delta %s", id)
}

func mustMilestones(plan *continuity.Plan) []continuity.Milestone {
	items, _ := plan.Milestones()
	return items
}

func mustOutcomes(plan *continuity.Plan) []continuity.Outcome {
	items, _ := plan.Outcomes()
	return items
}

func mustDecisions(plan *continuity.Plan) []continuity.DecisionCard {
	items, _ := plan.Decisions()
	return items
}
