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

	"continuity/internal/continuity"
)

func statusCommand(args []string) error {
	root, err := parseRepo(args, "status")
	if err != nil {
		return err
	}
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return err
	}
	completions, err := continuity.ListCompletions(root)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	fmt.Printf("Coordination status — %s\n", root)
	if len(claims) == 0 {
		fmt.Println("Active claims: none")
	} else {
		fmt.Println("Active claims:")
		for _, claim := range claims {
			lease := "active"
			if claim.Stale(now) {
				lease = "STALE"
			} else {
				lease = "expires " + claim.LeaseExpires.Format(time.RFC3339)
			}
			fmt.Printf("- %s | owner: %s | worktree: %s | scope: %s | %s\n", claim.TaskID, claim.Owner, claim.Worktree, strings.Join(claim.Scope, ","), lease)
			if claim.NextAction != "" {
				fmt.Println("  next:", claim.NextAction)
			}
		}
	}
	if len(completions) == 0 {
		fmt.Println("Completed awaiting integration: none")
	} else {
		fmt.Println("Completed awaiting integration:")
		for _, completion := range completions {
			fmt.Printf("- %s | commit: %s | %s\n", completion.TaskID, completion.Commit, completion.Summary)
		}
	}
	return nil
}

func heartbeatCommand(args []string) error {
	flags := flag.NewFlagSet("heartbeat", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "active task ID; inferred from this worktree when omitted")
	session := flags.String("session", "", "Codex session ID from continuity context")
	owner := flags.String("owner", "", "stable worker identity")
	if err := flags.Parse(args); err != nil {
		return err
	}
	claimOwner := strings.TrimSpace(*owner)
	if claimOwner == "" {
		claimOwner = ownerForSession(*session)
	}
	if claimOwner == "" {
		return errors.New("--session or --owner is required")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	claim, err := continuity.HeartbeatClaim(root, *id, claimOwner, root)
	if err != nil {
		return err
	}
	fmt.Printf("heartbeat %s until %s\n", claim.TaskID, claim.LeaseExpires.Format(time.RFC3339))
	return nil
}

func releaseCommand(args []string) error {
	flags := flag.NewFlagSet("release", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "active task ID; inferred from this worktree when omitted")
	session := flags.String("session", "", "Codex session ID from continuity context")
	owner := flags.String("owner", "", "stable worker identity")
	forceStale := flags.Bool("force-stale", false, "release only a stale claim without its owner identity")
	if err := flags.Parse(args); err != nil {
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
	if claimOwner == "" && !*forceStale {
		return errors.New("--session or --owner is required unless --force-stale is used")
	}
	if *forceStale {
		if *id == "" {
			return errors.New("--id is required with --force-stale")
		}
		if err := continuity.RecoverStaleClaim(root, *id); err != nil {
			return err
		}
		fmt.Println("recovered stale claim", *id)
		return nil
	}
	if err := continuity.ReleaseClaim(root, *id, claimOwner, root); err != nil {
		return err
	}
	fmt.Println("released", *id)
	return nil
}

func recoverLockCommand(args []string) error {
	flags := flag.NewFlagSet("recover-lock", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	force := flags.Bool("force", false, "confirm recovery of an abandoned coordination lock")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !*force {
		return errors.New("--force is required after confirming no continuity command is running")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	if err := continuity.RecoverCoordinationLock(root); err != nil {
		return err
	}
	fmt.Println("recovered coordination lock")
	return nil
}

func contextCommand(args []string) error {
	flags := flag.NewFlagSet("context", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	session := flags.String("session", "", "Codex session ID (adapter convenience)")
	owner := flags.String("owner", "", "stable harness-neutral worker identity")
	scope := flags.String("scope", "", "comma-separated repository paths whose durable capsules should be selected")
	limit := flags.Int("limit", 3200, "maximum context bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *limit <= 0 {
		return errors.New("--limit must be positive")
	}
	requestedScopes, err := parseContextScopes(*scope)
	if err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	contextOwner := contextOwnerFor(*owner, *session)
	context, err := compactContext(root, contextOwner, *limit, requestedScopes)
	if err != nil {
		return err
	}
	fmt.Print(context)
	return nil
}

func compactContext(root, owner string, limit int, requestedScopes []string) (string, error) {
	if limit <= 0 {
		return "", errors.New("context limit must be positive")
	}
	state, err := loadState(root)
	if err != nil {
		return "", err
	}
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return "", err
	}
	completions, err := continuity.ListCompletions(root)
	if err != nil {
		return "", err
	}
	current, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString("Continuity live coordination\n")
	if owner != "" {
		builder.WriteString("Your worker identity: ")
		builder.WriteString(owner)
		builder.WriteString("\n")
	}
	if current == nil {
		builder.WriteString("This worktree has no active writer claim. Before any edit, run continuity start with this worker identity.\n")
	} else if current.Stale(time.Now().UTC()) {
		fmt.Fprintf(&builder, "STOP: this worktree claim for %s owned by %s is stale. Inspect continuity status, then explicitly heartbeat it as the original owner or recover/release it before editing.\n", current.TaskID, current.Owner)
	} else if owner == "" {
		fmt.Fprintf(&builder, "STOP: this worktree is claimed by %s for %s, but this session has no worker identity. Do not edit until it identifies itself and owns a claim.\n", current.Owner, current.TaskID)
	} else if current.Owner != owner {
		fmt.Fprintf(&builder, "STOP: this worktree is claimed by %s for %s. Do not edit or overwrite its changes.\n", current.Owner, current.TaskID)
	} else {
		fmt.Fprintf(&builder, "Your active claim: %s | scope: %s\n", current.TaskID, strings.Join(current.Scope, ","))
		if current.Goal != "" {
			fmt.Fprintf(&builder, "Goal: %s\n", current.Goal)
		}
		if current.NextAction != "" {
			fmt.Fprintf(&builder, "Next action: %s\n", current.NextAction)
		}
		if current.Risk != "" && current.Risk != "none" {
			fmt.Fprintf(&builder, "Risk: %s\n", current.Risk)
		}
	}
	otherClaims := make([]continuity.Claim, 0, len(claims))
	for _, claim := range claims {
		if current == nil || claim.TaskID != current.TaskID {
			otherClaims = append(otherClaims, claim)
		}
	}
	if len(otherClaims) > 0 {
		builder.WriteString("Other active work:\n")
		for _, claim := range otherClaims {
			fmt.Fprintf(&builder, "- %s owned by %s; scope %s\n", claim.TaskID, claim.Owner, strings.Join(claim.Scope, ","))
		}
	}
	if len(completions) > 0 {
		builder.WriteString("Completed work awaiting merge/integration:\n")
		for _, completion := range completions {
			fmt.Fprintf(&builder, "- %s at %s\n", completion.TaskID, completion.Commit)
		}
	}
	if ready, err := continuity.ReadyTasks(state.Section("Ready Queue")); err == nil {
		claimed := map[string]bool{}
		for _, claim := range claims {
			claimed[claim.TaskID] = true
		}
		for _, task := range ready {
			if !claimed[task.ID] {
				fmt.Fprintf(&builder, "Ready approved task: %s — %s\n", task.ID, task.Title)
				break
			}
		}
	}
	builder.WriteString("Project intent: ")
	builder.WriteString(singleLineSnippet(state.Section("Product Intent"), 420))
	builder.WriteString("\nConstraints: ")
	builder.WriteString(singleLineSnippet(state.Section("Constraints and Approval Boundaries"), 700))
	selectionScopes := requestedScopes
	if len(selectionScopes) == 0 && current != nil {
		selectionScopes = current.Scope
	}
	capsules, capsuleErr := state.ContextCapsules()
	if capsuleErr != nil {
		return "", fmt.Errorf("read Context Capsules: %w", capsuleErr)
	}
	if selected := continuity.SelectContextCapsules(capsules, selectionScopes); len(selected) > 0 {
		builder.WriteString("\nSelected durable context capsules:\n")
		for _, capsule := range selected {
			builder.WriteString(capsule.Render())
		}
	}
	builder.WriteString("\nUse the durable state for decisions; use the live claim board for concurrent ownership.\n")
	return boundText(builder.String(), limit), nil
}

// livePulse is intentionally much smaller than the SessionStart packet. It is
// injected before a later user prompt so an already-open session learns that a
// sibling session claimed or released work without repeatedly receiving the
// durable project index.
func livePulse(root, owner string) (string, error) {
	current, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		return "", err
	}
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return "", err
	}
	completions, err := continuity.ListCompletions(root)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString("Continuity pulse: ")
	switch {
	case current == nil:
		builder.WriteString("this worktree has no active writer claim; claim before editing.")
	case current.Stale(time.Now().UTC()):
		fmt.Fprintf(&builder, "STOP — %s is stale under %s; inspect status and explicitly recover or resume it before editing.", current.TaskID, current.Owner)
	case owner == "":
		fmt.Fprintf(&builder, "STOP — this worktree is claimed by %s for %s, but this session has no worker identity.", current.Owner, current.TaskID)
	case current.Owner != owner:
		fmt.Fprintf(&builder, "STOP — this worktree is claimed by %s for %s; do not edit it.", current.Owner, current.TaskID)
	default:
		fmt.Fprintf(&builder, "you own %s (scope %s).", current.TaskID, strings.Join(current.Scope, ","))
	}
	others := 0
	for _, claim := range claims {
		if current == nil || claim.TaskID != current.TaskID {
			others++
		}
	}
	if others > 0 {
		fmt.Fprintf(&builder, " %d other live claim(s) exist in linked worktrees.", others)
	}
	if len(completions) > 0 {
		fmt.Fprintf(&builder, " %d completed result(s) await integration.", len(completions))
	}
	builder.WriteByte('\n')
	return boundText(builder.String(), 700), nil
}

func integrateCommand(args []string) error {
	flags := flag.NewFlagSet("integrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "completed task ID")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *id == "" {
		return errors.New("--id is required")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	var integrated continuity.Completion
	err = continuity.WithCoordinationLock(root, func() error {
		completions, err := continuity.ListCompletions(root)
		if err != nil {
			return err
		}
		found := false
		for _, completion := range completions {
			if completion.TaskID == *id {
				integrated = completion
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("no completed local record exists for %s", *id)
		}
		ancestor, err := continuity.CommitIsAncestor(root, integrated.Commit)
		if err != nil {
			return err
		}
		if !ancestor {
			return fmt.Errorf("commit %s is not in this checkout; merge or hand off the worker result before integrating %s", integrated.Commit, *id)
		}
		state, err := loadState(root)
		if err != nil {
			return err
		}
		if result := state.Validate(continuity.DefaultStateBudget); !result.OK() {
			return fmt.Errorf("PROJECT_STATE.md is invalid: %s", strings.Join(result.Errors, "; "))
		}
		snapshot := continuity.Snapshot(root)
		if state.Meta["branch"] != "" && state.Meta["branch"] != "unborn" && snapshot.Branch != state.Meta["branch"] {
			return fmt.Errorf("integration must run on the durable index branch %q, not %q; merge or hand off first", state.Meta["branch"], snapshot.Branch)
		}
		if err := requireSingleLine(map[string]string{"completion summary": integrated.Summary, "completion commit": integrated.Commit}); err != nil {
			return fmt.Errorf("invalid completion record: %w", err)
		}
		for _, evidence := range integrated.Evidence {
			if err := requireSingleLine(map[string]string{"completion evidence": evidence}); err != nil {
				return fmt.Errorf("invalid completion record: %w", err)
			}
		}
		tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
		if err != nil {
			return err
		}
		var task continuity.Task
		found = false
		for _, candidate := range tasks {
			if candidate.ID == *id {
				task = candidate
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("task %s is not in Ready Queue", *id)
		}
		if task.Status != "x" {
			queue, err := continuity.UpdateTaskStatus(state.Section("Ready Queue"), *id, "x")
			if err != nil {
				return err
			}
			if err := state.ReplaceSection("Ready Queue", queue); err != nil {
				return err
			}
			recent := strings.TrimSpace(state.Section("Recent Material Changes"))
			recent = "- Integrated " + *id + " at " + integrated.Commit + ": " + integrated.Summary + "\n" + recent
			if err := state.ReplaceSection("Recent Material Changes", recent); err != nil {
				return err
			}
			verification := strings.TrimSpace(state.Section("Verification"))
			for _, evidence := range integrated.Evidence {
				verification = "- " + evidence + "\n" + verification
			}
			if err := state.ReplaceSection("Verification", verification); err != nil {
				return err
			}
			if err := synchronize(state, root); err != nil {
				return err
			}
			if err := state.Save(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := continuity.RemoveCompletion(root, *id); err != nil {
		return err
	}
	fmt.Printf("integrated %s from %s\n", *id, integrated.Commit)
	return nil
}

type codexHookInput struct {
	SessionID string          `json:"session_id"`
	Cwd       string          `json:"cwd"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
}

func codexHookCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("codex-hook requires session-start, subagent-start, user-prompt, or pretool")
	}
	phase := args[0]
	flags := flag.NewFlagSet("codex-hook "+phase, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", "", "repository path")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	input := codexHookInput{}
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
		return nil // Hooks must not interfere with unrelated directories.
	}
	owner := ownerForSession(input.SessionID)
	switch phase {
	case "session-start", "subagent-start":
		heartbeatOwnLiveClaim(root, owner)
		context, contextErr := compactContext(root, owner, 1800, nil)
		if contextErr != nil {
			context = "Continuity state could not be loaded: " + contextErr.Error()
		}
		event := "SessionStart"
		if phase == "subagent-start" {
			event = "SubagentStart"
			context += "Subagent safety: Codex child hooks inherit the parent session identity. Stay read-only in this checkout; a child that needs to write must use a separate worktree and its own claim.\n"
		}
		return emitHookContext(event, context)
	case "user-prompt":
		heartbeatOwnLiveClaim(root, owner)
		pulse, pulseErr := livePulse(root, owner)
		if pulseErr != nil {
			pulse = "Continuity pulse unavailable: " + pulseErr.Error()
		}
		return emitHookContext("UserPromptSubmit", pulse)
	case "pretool":
		switch input.ToolName {
		case "", "apply_patch", "Edit", "Write":
			if err := guardCodexWrite(root, owner); err != nil {
				return emitHookDeny(err.Error())
			}
		case "Bash":
			if err := guardCodexBash(root, owner, input.bashCommand()); err != nil {
				return emitHookDeny(err.Error())
			}
		default:
			return nil
		}
		return nil
	default:
		return fmt.Errorf("unknown codex hook phase %q", phase)
	}
}

func heartbeatOwnLiveClaim(root, owner string) {
	if owner == "" {
		return
	}
	claim, err := continuity.FindClaimForWorktree(root)
	if err == nil && claim != nil && claim.Owner == owner && !claim.Stale(time.Now().UTC()) {
		_, _ = continuity.HeartbeatClaim(root, claim.TaskID, owner, root)
	}
}

func (input codexHookInput) bashCommand() string {
	var tool struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input.ToolInput, &tool); err != nil {
		return ""
	}
	return tool.Command
}

// guardCodexBash permits a deliberately small set of inspection and
// Continuity-lifecycle commands without a worktree claim. Everything else is
// treated as potentially mutating because shell syntax is too expressive to
// classify safely. Once an owner has a live claim, normal Bash work proceeds
// and refreshes that claim; a non-owner remains limited to inspection.
func guardCodexBash(root, owner, command string) error {
	if isContinuityCommand(command) || isReadOnlyBashCommand(command) {
		return nil
	}
	return guardCodexWrite(root, owner)
}

func isContinuityCommand(command string) bool {
	parts, ok := simpleBashWords(command)
	if !ok || len(parts) == 0 {
		return false
	}
	if parts[0] == "continuity" {
		return len(parts) >= 2 && isContinuityLifecycleVerb(parts[1])
	}
	if len(parts) >= 4 && parts[0] == "go" && parts[1] == "run" && parts[2] == "./cmd/continuity" {
		return isContinuityLifecycleVerb(parts[3])
	}
	return false
}

func isContinuityLifecycleVerb(value string) bool {
	switch value {
	case "brief", "check", "context", "finalize", "heartbeat", "integrate", "ready", "reconcile", "recover-lock", "release", "start", "status":
		return true
	default:
		return false
	}
}

func isReadOnlyBashCommand(command string) bool {
	parts, ok := simpleBashWords(command)
	if !ok || len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "pwd", "ls", "cat", "head", "tail", "rg", "grep":
		return true
	case "git":
		if len(parts) < 2 {
			return false
		}
		switch parts[1] {
		case "blame", "branch", "diff", "grep", "log", "ls-files", "remote", "rev-parse", "show", "status":
			return true
		default:
			return false
		}
	case "go":
		return len(parts) >= 2 && (parts[1] == "env" || parts[1] == "list" || parts[1] == "version")
	default:
		return false
	}
}

func simpleBashWords(command string) ([]string, bool) {
	command = strings.TrimSpace(command)
	if command == "" || strings.ContainsAny(command, "\r\n;&|<>`$()") {
		return nil, false
	}
	return strings.Fields(command), true
}

func guardCodexWrite(root, owner string) error {
	if _, err := os.Stat(filepath.Join(root, continuity.StateFilename)); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if owner == "" {
		return errors.New("Continuity cannot identify this Codex session; start or resume the session so it receives a worker identity before editing")
	}
	claim, err := continuity.FindClaimForWorktree(root)
	if err != nil {
		return err
	}
	if claim == nil {
		return fmt.Errorf("no active claim for this worktree; run continuity start with --session %s before editing", strings.TrimPrefix(owner, "codex:"))
	}
	if claim.Owner != owner {
		return fmt.Errorf("this worktree is claimed by %s for %s; do not edit it from session %s", claim.Owner, claim.TaskID, owner)
	}
	if claim.Stale(time.Now().UTC()) {
		return fmt.Errorf("claim %s is stale; inspect continuity status and explicitly recover or release it before editing", claim.TaskID)
	}
	_, err = continuity.HeartbeatClaim(root, claim.TaskID, owner, root)
	return err
}

func emitHookContext(event, additionalContext string) error {
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": additionalContext,
		},
	})
}

func emitHookDeny(reason string) error {
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       "deny",
			"permissionDecisionReason": reason,
		},
	})
}

func ownerForSession(session string) string {
	session = strings.TrimSpace(session)
	if session == "" {
		return ""
	}
	if strings.HasPrefix(session, "codex:") {
		return session
	}
	return "codex:" + session
}

func contextOwnerFor(owner, session string) string {
	if owner = strings.TrimSpace(owner); owner != "" {
		return owner
	}
	return ownerForSession(session)
}

func defaultOwner() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "local"
	}
	return fmt.Sprintf("local:%s:%d", host, os.Getpid())
}

func parseScope(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func parseContextScopes(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	scopes := []string{}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, errors.New("context scope cannot be empty")
		}
		if filepath.IsAbs(item) {
			return nil, fmt.Errorf("context scope %q must be repository-relative", item)
		}
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("context scope %q escapes the repository", item)
		}
		if !seen[clean] {
			seen[clean] = true
			scopes = append(scopes, clean)
		}
	}
	return scopes, nil
}

func parseEvidence(value string) []string {
	items := []string{}
	for _, item := range strings.Split(value, ";") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return items
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
	return boundText(value, limit)
}

func boundText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit < 4 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
