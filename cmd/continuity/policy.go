package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"continuity/internal/continuity"
)

func policyCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: continuity policy add|check|status ...")
	}
	switch args[0] {
	case "add":
		return policyAddCommand(args[1:])
	case "check":
		return policyCheckCommand(args[1:])
	case "status":
		return policyStatusCommand(args[1:])
	default:
		return errors.New("usage: continuity policy add|check|status ...")
	}
}

func policyAddCommand(args []string) error {
	flags := flag.NewFlagSet("policy add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "policy ID")
	scope := flags.String("scope", "", "comma-separated repository-relative scope")
	phase := flags.String("phase", "finalize", "lifecycle phase")
	command := flags.String("command", "", "approved shell command")
	reason := flags.String("reason", "", "why this check is required")
	approved := flags.Bool("approved", false, "confirm explicit user approval for this executable policy")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*approved {
		return errors.New("--approved is required; Continuity cannot infer permission to execute a new policy command")
	}
	if err := requireSingleLine(map[string]string{
		"--id":      *id,
		"--scope":   *scope,
		"--phase":   *phase,
		"--command": *command,
		"--reason":  *reason,
	}); err != nil {
		return err
	}
	paths, err := parseContextScopes(*scope)
	if err != nil {
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
		document, err := loadOptionalPolicyDocument(root)
		if err != nil {
			return err
		}
		if document == nil {
			document = continuity.NewPolicyDocument(continuity.PolicyPath(root))
		}
		policy := continuity.Policy{ID: *id, Scope: paths, Phase: *phase, Command: *command, Reason: *reason}
		if err := document.Add(policy); err != nil {
			return err
		}
		if err := document.Save(); err != nil {
			return err
		}
		state, err := loadState(root)
		if err != nil {
			return err
		}
		if _, err := synchronizeWithAging(state, root); err != nil {
			return err
		}
		if err := state.Save(); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(policy): add "+policy.ID, continuity.PolicyFilename, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("added policy %s and committed %s\n", *id, commit)
	return nil
}

func policyCheckCommand(args []string) error {
	flags := flag.NewFlagSet("policy check", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	scope := flags.String("scope", ".", "comma-separated repository-relative scope")
	phase := flags.String("phase", "finalize", "lifecycle phase")
	if err := flags.Parse(args); err != nil {
		return err
	}
	scopes, err := parseContextScopes(*scope)
	if err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	results, err := runPolicyChecks(root, scopes, *phase)
	for _, result := range results {
		status := "PASS"
		if result.Err != nil {
			status = "FAIL"
		}
		fmt.Printf("%s %s | scope: %s | %s\n", status, result.Policy.ID, strings.Join(result.Policy.Scope, ","), result.Policy.Reason)
	}
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Println("No matching policy checks.")
	}
	return nil
}

func policyStatusCommand(args []string) error {
	root, err := parseRepo(args, "policy status")
	if err != nil {
		return err
	}
	document, err := loadOptionalPolicyDocument(root)
	if err != nil {
		return err
	}
	if document == nil || len(document.Policies) == 0 {
		fmt.Println("Policy checks: none configured")
		return nil
	}
	for _, policy := range document.Policies {
		fmt.Printf("- %s | phase: %s | scope: %s | %s\n", policy.ID, policy.Phase, strings.Join(policy.Scope, ","), policy.Reason)
	}
	return nil
}

type policyCheckResult struct {
	Policy continuity.Policy
	Err    error
}

// runPolicyChecks executes only user-approved commands from the committed
// policy file. It deliberately does not print command output: that output can
// contain application data or secrets and belongs in the developer's terminal
// when they intentionally run the policy directly.
func runPolicyChecks(root string, scopes []string, phase string) ([]policyCheckResult, error) {
	document, err := loadOptionalPolicyDocument(root)
	if err != nil {
		return nil, err
	}
	if document == nil {
		return nil, nil
	}
	results := []policyCheckResult{}
	for _, policy := range continuity.PoliciesForScopes(document.Policies, scopes, phase) {
		command := shellPolicyCommand(policy.Command)
		command.Dir = root
		_, commandErr := command.CombinedOutput()
		result := policyCheckResult{Policy: policy, Err: commandErr}
		results = append(results, result)
		if commandErr != nil {
			return results, fmt.Errorf("policy %s failed (%s); inspect the configured command locally before finalizing", policy.ID, policy.Reason)
		}
	}
	return results, nil
}

func enforceFinalizationPolicies(root string, claim *continuity.Claim) ([]string, error) {
	if claim == nil {
		return nil, nil
	}
	results, err := runPolicyChecks(root, claim.Scope, "finalize")
	if err != nil {
		failedID := "configured policy"
		for _, result := range results {
			if result.Err != nil {
				failedID = result.Policy.ID
				break
			}
		}
		checkpoint := continuity.CheckpointRequest{
			TaskID:     claim.TaskID,
			Owner:      claim.Owner,
			Worktree:   root,
			Summary:    "Policy gate " + failedID + " failed.",
			NextAction: "Inspect policy " + failedID + ", gather new evidence, then rerun its focused check. If the task cannot proceed, release the claim and record a durable blocker card.",
			Risk:       "Policy gate " + failedID + " failed.",
		}
		if _, checkpointErr := continuity.CheckpointClaim(root, checkpoint); checkpointErr != nil {
			return nil, fmt.Errorf("%w; also could not checkpoint the policy failure: %v", err, checkpointErr)
		}
		return nil, fmt.Errorf("%w; claim checkpointed and finalization remains blocked", err)
	}
	evidence := make([]string, 0, len(results))
	for _, result := range results {
		evidence = append(evidence, "policy "+result.Policy.ID+": pass")
	}
	return evidence, nil
}

func shellPolicyCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

func loadOptionalPolicyDocument(root string) (*continuity.PolicyDocument, error) {
	document, err := continuity.LoadPolicyDocument(continuity.PolicyPath(root))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return document, err
}
