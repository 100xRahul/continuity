package continuity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	coordinationDirectory = "continuity"
	claimLease            = 45 * time.Minute
	lockWait              = 2 * time.Second
	staleLockAge          = 5 * time.Minute
)

// Claim is short-lived, local coordination state. It deliberately lives in
// Git's common directory rather than a worktree so linked worktrees on one
// machine see the same ownership information without creating merge conflicts.
type Claim struct {
	Schema          int       `json:"schema"`
	TaskID          string    `json:"task_id"`
	Owner           string    `json:"owner"`
	Worktree        string    `json:"worktree"`
	Branch          string    `json:"branch"`
	BaseCommit      string    `json:"base_commit,omitempty"`
	Scope           []string  `json:"scope"`
	Goal            string    `json:"goal"`
	Acceptance      string    `json:"acceptance"`
	Mode            string    `json:"mode"`
	NextAction      string    `json:"next_action"`
	Validation      string    `json:"validation"`
	SelectionReason string    `json:"selection_reason,omitempty"`
	Progress        string    `json:"progress,omitempty"`
	Risk            string    `json:"risk,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	HeartbeatAt     time.Time `json:"heartbeat_at"`
	LeaseExpires    time.Time `json:"lease_expires"`
	Nonce           string    `json:"nonce"`
}

func (c Claim) Stale(now time.Time) bool {
	return !c.LeaseExpires.IsZero() && c.LeaseExpires.Before(now)
}

// Completion records a worker's terminal result until an integration checkout
// confirms that the referenced commit is present and projects it into
// PROJECT_STATE.md.
type Completion struct {
	Schema          int       `json:"schema"`
	TaskID          string    `json:"task_id"`
	Owner           string    `json:"owner"`
	Worktree        string    `json:"worktree"`
	Branch          string    `json:"branch"`
	BaseCommit      string    `json:"base_commit,omitempty"`
	Scope           []string  `json:"scope,omitempty"`
	Goal            string    `json:"goal,omitempty"`
	Acceptance      string    `json:"acceptance,omitempty"`
	NextAction      string    `json:"next_action,omitempty"`
	Validation      string    `json:"validation,omitempty"`
	SelectionReason string    `json:"selection_reason,omitempty"`
	Commit          string    `json:"commit"`
	Summary         string    `json:"summary"`
	Evidence        []string  `json:"evidence"`
	CompletedAt     time.Time `json:"completed_at"`
}

type ClaimRequest struct {
	TaskID          string
	Owner           string
	Worktree        string
	Branch          string
	BaseCommit      string
	Scope           []string
	Goal            string
	Acceptance      string
	Mode            string
	NextAction      string
	Validation      string
	SelectionReason string
	Lease           time.Duration
}

type CheckpointRequest struct {
	TaskID     string
	Owner      string
	Worktree   string
	Summary    string
	NextAction string
	Risk       string
}

type CompletionRequest struct {
	TaskID   string
	Owner    string
	Worktree string
	Commit   string
	Summary  string
	Evidence []string
}

// ClaimConflictError is returned when a live claim makes an edit unsafe.
// Callers can show its compact fields directly to a newly started agent.
type ClaimConflictError struct {
	Reason string
	Claim  Claim
}

func (e *ClaimConflictError) Error() string {
	return fmt.Sprintf("%s: %s is owned by %s in %s", e.Reason, e.Claim.TaskID, e.Claim.Owner, e.Claim.Worktree)
}

func GitCommonDir(directory string) (string, error) {
	root, err := GitRoot(directory)
	if err != nil {
		return "", err
	}
	output, err := git(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	common := strings.TrimSpace(output)
	if common == "" {
		return "", errors.New("Git common directory is empty")
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(root, common)
	}
	return filepath.Clean(common), nil
}

func CoordinationRoot(directory string) (string, error) {
	common, err := GitCommonDir(directory)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, coordinationDirectory), nil
}

func ListClaims(directory string) ([]Claim, error) {
	store, err := CoordinationRoot(directory)
	if err != nil {
		return nil, err
	}
	claims, _, err := listClaims(store)
	return claims, err
}

// ListDamagedClaims reports claim directories whose records cannot be parsed.
// Callers that display coordination state should surface these, because they
// are invisible to ListClaims yet still occupy their task ID.
func ListDamagedClaims(directory string) ([]string, error) {
	store, err := CoordinationRoot(directory)
	if err != nil {
		return nil, err
	}
	_, damaged, err := listClaims(store)
	return damaged, err
}

func ListCompletions(directory string) ([]Completion, error) {
	store, err := CoordinationRoot(directory)
	if err != nil {
		return nil, err
	}
	return listCompletions(store)
}

func ClaimTask(directory string, request ClaimRequest) (Claim, error) {
	var created Claim
	err := withCoordinationLock(directory, func(store string) error {
		root, err := GitRoot(directory)
		if err != nil {
			return err
		}
		if request.TaskID == "" || request.Owner == "" {
			return errors.New("task ID and owner are required for a claim")
		}
		if !validCoordinationTaskID(request.TaskID) {
			return fmt.Errorf("invalid task ID %q", request.TaskID)
		}
		if !singleLine(request.Owner) || !singleLineOrEmpty(request.BaseCommit) || !singleLineOrEmpty(request.Goal) || !singleLineOrEmpty(request.Acceptance) || !singleLineOrEmpty(request.NextAction) || !singleLineOrEmpty(request.Validation) || !singleLineOrEmpty(request.SelectionReason) {
			return errors.New("claim fields must be single-line text")
		}
		branch := strings.TrimSpace(request.Branch)
		worktree := ""
		if request.Worktree == "" {
			worktree, err = canonicalPath(root)
			if err != nil {
				return err
			}
		} else {
			worktree, err = canonicalPath(request.Worktree)
			if err != nil {
				return err
			}
		}
		scope, err := normalizeScopes(request.Scope)
		if err != nil {
			return err
		}
		if completed, err := completionForTask(store, request.TaskID); err != nil {
			return err
		} else if completed != nil {
			return fmt.Errorf("task %s has a completed local result awaiting integration from commit %s", request.TaskID, completed.Commit)
		}
		claims, damaged, err := listClaims(store)
		if err != nil {
			return err
		}
		// Granting is the one path that must fail closed: an unreadable record
		// could hold any task, branch, or scope, so exclusivity is unprovable.
		if len(damaged) > 0 {
			return fmt.Errorf("cannot prove exclusivity while claim record(s) %s are unreadable; clear each with continuity release --force-stale --id <task> before claiming", strings.Join(damaged, ", "))
		}
		for _, existing := range claims {
			if existing.TaskID == request.TaskID {
				return &ClaimConflictError{Reason: "task is already claimed", Claim: existing}
			}
			if samePath(existing.Worktree, worktree) {
				return &ClaimConflictError{Reason: "worktree already has an active writer", Claim: existing}
			}
			if sameBranch(existing.Branch, branch) {
				return &ClaimConflictError{Reason: "branch already has an active writer", Claim: existing}
			}
			if scopesOverlap(existing.Scope, scope) {
				return &ClaimConflictError{Reason: "edit scope overlaps an active claim", Claim: existing}
			}
		}
		lease := request.Lease
		if lease <= 0 {
			lease = claimLease
		}
		now := time.Now().UTC()
		nonce, err := randomNonce()
		if err != nil {
			return err
		}
		claimDir := filepath.Join(store, "claims", request.TaskID)
		if err := os.MkdirAll(filepath.Dir(claimDir), 0o700); err != nil {
			return err
		}
		if err := os.Mkdir(claimDir, 0o700); err != nil {
			if errors.Is(err, os.ErrExist) {
				if existing, readErr := readClaim(filepath.Join(claimDir, "claim.json")); readErr == nil {
					return &ClaimConflictError{Reason: "task is already claimed", Claim: existing}
				}
				return fmt.Errorf("task %s has a claim directory that needs recovery", request.TaskID)
			}
			return err
		}
		created = Claim{
			Schema:          1,
			TaskID:          request.TaskID,
			Owner:           request.Owner,
			Worktree:        worktree,
			Branch:          branch,
			BaseCommit:      request.BaseCommit,
			Scope:           scope,
			Goal:            request.Goal,
			Acceptance:      request.Acceptance,
			Mode:            request.Mode,
			NextAction:      request.NextAction,
			Validation:      request.Validation,
			SelectionReason: request.SelectionReason,
			CreatedAt:       now,
			HeartbeatAt:     now,
			LeaseExpires:    now.Add(lease),
			Nonce:           nonce,
		}
		if err := writeJSONAtomic(filepath.Join(claimDir, "claim.json"), created, 0o600); err != nil {
			_ = os.Remove(claimDir)
			return err
		}
		return nil
	})
	return created, err
}

func sameBranch(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && left == right
}

func HeartbeatClaim(directory, taskID, owner, worktree string) (Claim, error) {
	return updateClaim(directory, taskID, owner, worktree, func(claim *Claim) error {
		now := time.Now().UTC()
		claim.HeartbeatAt = now
		claim.LeaseExpires = now.Add(claimLease)
		return nil
	})
}

func CheckpointClaim(directory string, request CheckpointRequest) (Claim, error) {
	// Checkpoint both renews the lease and overwrites progress and next action.
	// Without an owner it would match on worktree alone, letting any process in
	// that checkout extend and destroy another owner's restart state.
	if strings.TrimSpace(request.Owner) == "" {
		return Claim{}, errors.New("checkpoint requires --owner or --session so it cannot overwrite another worker's claim")
	}
	if !singleLine(request.Summary) || !singleLine(request.NextAction) || !singleLineOrEmpty(request.Risk) {
		return Claim{}, errors.New("checkpoint fields must be single-line text")
	}
	return updateClaim(directory, request.TaskID, request.Owner, request.Worktree, func(claim *Claim) error {
		now := time.Now().UTC()
		claim.Progress = request.Summary
		claim.NextAction = request.NextAction
		claim.Risk = request.Risk
		claim.HeartbeatAt = now
		claim.LeaseExpires = now.Add(claimLease)
		return nil
	})
}

func CompleteClaim(directory string, request CompletionRequest) (Completion, error) {
	var completion Completion
	err := withCoordinationLock(directory, func(store string) error {
		claim, claimPath, err := locateClaim(store, request.TaskID, request.Owner, request.Worktree)
		if err != nil {
			return err
		}
		if request.Commit == "" || request.Summary == "" || len(request.Evidence) == 0 {
			return errors.New("completion requires a commit, summary, and at least one verification evidence item")
		}
		if !singleLine(request.Commit) || !singleLine(request.Summary) {
			return errors.New("completion fields must be single-line text")
		}
		for _, evidence := range request.Evidence {
			if !singleLine(evidence) {
				return errors.New("completion evidence must be single-line text")
			}
		}
		completion = Completion{
			Schema:          1,
			TaskID:          claim.TaskID,
			Owner:           claim.Owner,
			Worktree:        claim.Worktree,
			Branch:          claim.Branch,
			BaseCommit:      claim.BaseCommit,
			Scope:           append([]string(nil), claim.Scope...),
			Goal:            claim.Goal,
			Acceptance:      claim.Acceptance,
			NextAction:      claim.NextAction,
			Validation:      claim.Validation,
			SelectionReason: claim.SelectionReason,
			Commit:          request.Commit,
			Summary:         request.Summary,
			Evidence:        append([]string(nil), request.Evidence...),
			CompletedAt:     time.Now().UTC(),
		}
		completionPath := filepath.Join(store, "completed", claim.TaskID+".json")
		if err := os.MkdirAll(filepath.Dir(completionPath), 0o700); err != nil {
			return err
		}
		if err := writeJSONAtomic(completionPath, completion, 0o600); err != nil {
			return err
		}
		if err := os.Remove(claimPath); err != nil {
			return err
		}
		return os.Remove(filepath.Dir(claimPath))
	})
	return completion, err
}

func ReleaseClaim(directory, taskID, owner, worktree string) error {
	return withCoordinationLock(directory, func(store string) error {
		_, claimPath, err := locateClaim(store, taskID, owner, worktree)
		if err != nil {
			return err
		}
		if err := os.Remove(claimPath); err != nil {
			return err
		}
		return os.Remove(filepath.Dir(claimPath))
	})
}

// RecoverStaleClaim removes an expired claim from any local worktree. It is
// deliberately separate from ordinary release so a live worker cannot be
// displaced merely because another session wants the task.
func RecoverStaleClaim(directory, taskID string) error {
	if !validCoordinationTaskID(taskID) {
		return fmt.Errorf("invalid task ID %q", taskID)
	}
	return withCoordinationLock(directory, func(store string) error {
		claimDir := filepath.Join(store, "claims", taskID)
		if _, err := os.Stat(claimDir); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errors.New("no matching active claim")
			}
			return err
		}
		claim, err := readClaim(filepath.Join(claimDir, "claim.json"))
		if err != nil {
			// A record that cannot be parsed carries no lease to respect, and
			// clearing it is precisely what recovery is for.
			return os.RemoveAll(claimDir)
		}
		if !claim.Stale(time.Now().UTC()) {
			return fmt.Errorf("claim %s is still live", claim.TaskID)
		}
		return os.RemoveAll(claimDir)
	})
}

// RecoverCoordinationLock removes an abandoned internal mutex only after it
// has exceeded the stale threshold. It is explicit because automatically
// deleting a lock could race a slow but still-live integration operation.
func RecoverCoordinationLock(directory string) error {
	store, err := CoordinationRoot(directory)
	if err != nil {
		return err
	}
	lock := filepath.Join(store, "locks", "coordination.lock")
	info, err := os.Stat(lock)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if time.Since(info.ModTime()) <= staleLockAge {
		return errors.New("coordination lock is recent; retry shortly instead of recovering it")
	}
	return os.Remove(lock)
}

func RemoveCompletion(directory, taskID string) error {
	return withCoordinationLock(directory, func(store string) error {
		path := filepath.Join(store, "completed", taskID+".json")
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	})
}

// WithCoordinationLock serializes brief mutations to the local coordination
// store. It is intended for integration code that needs to update a durable
// projection after checking local completion records.
func WithCoordinationLock(directory string, fn func() error) error {
	return withCoordinationLock(directory, func(_ string) error { return fn() })
}

func FindClaimForWorktree(directory string) (*Claim, error) {
	root, err := GitRoot(directory)
	if err != nil {
		return nil, err
	}
	worktree, err := canonicalPath(root)
	if err != nil {
		return nil, err
	}
	claims, err := ListClaims(directory)
	if err != nil {
		return nil, err
	}
	for _, claim := range claims {
		if samePath(claim.Worktree, worktree) {
			copy := claim
			return &copy, nil
		}
	}
	return nil, nil
}

func ClaimForTask(directory, taskID string) (*Claim, error) {
	claims, err := ListClaims(directory)
	if err != nil {
		return nil, err
	}
	for _, claim := range claims {
		if claim.TaskID == taskID {
			copy := claim
			return &copy, nil
		}
	}
	return nil, nil
}

func updateClaim(directory, taskID, owner, worktree string, mutate func(*Claim) error) (Claim, error) {
	var updated Claim
	err := withCoordinationLock(directory, func(store string) error {
		claim, path, err := locateClaim(store, taskID, owner, worktree)
		if err != nil {
			return err
		}
		if err := mutate(&claim); err != nil {
			return err
		}
		if err := writeJSONAtomic(path, claim, 0o600); err != nil {
			return err
		}
		updated = claim
		return nil
	})
	return updated, err
}

func locateClaim(store, taskID, owner, worktree string) (Claim, string, error) {
	claims, _, err := listClaims(store)
	if err != nil {
		return Claim{}, "", err
	}
	var requestedWorktree string
	if worktree != "" {
		requestedWorktree, err = canonicalPath(worktree)
		if err != nil {
			return Claim{}, "", err
		}
	}
	for _, claim := range claims {
		if taskID != "" && claim.TaskID != taskID {
			continue
		}
		if owner != "" && claim.Owner != owner {
			continue
		}
		if requestedWorktree != "" && !samePath(claim.Worktree, requestedWorktree) {
			continue
		}
		return claim, filepath.Join(store, "claims", claim.TaskID, "claim.json"), nil
	}
	return Claim{}, "", errors.New("no matching active claim")
}

// listClaims reports readable claims and the task IDs whose records could not be
// parsed. A damaged record must not hide every other claim: the commands that
// clear it (status, release --force-stale) read through this same path, so
// failing hard here would make the corruption unrecoverable.
func listClaims(store string) ([]Claim, []string, error) {
	entries, err := os.ReadDir(filepath.Join(store, "claims"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	claims := make([]Claim, 0, len(entries))
	var damaged []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		claim, err := readClaim(filepath.Join(store, "claims", entry.Name(), "claim.json"))
		if err != nil {
			damaged = append(damaged, entry.Name())
			continue
		}
		claims = append(claims, claim)
	}
	sort.Slice(claims, func(i, j int) bool { return claims[i].TaskID < claims[j].TaskID })
	sort.Strings(damaged)
	return claims, damaged, nil
}

func listCompletions(store string) ([]Completion, error) {
	entries, err := os.ReadDir(filepath.Join(store, "completed"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	completions := make([]Completion, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(store, "completed", entry.Name()))
		if err != nil {
			return nil, err
		}
		var completion Completion
		if err := json.Unmarshal(bytes, &completion); err != nil {
			return nil, fmt.Errorf("decode completion %s: %w", entry.Name(), err)
		}
		completions = append(completions, completion)
	}
	sort.Slice(completions, func(i, j int) bool { return completions[i].TaskID < completions[j].TaskID })
	return completions, nil
}

func completionForTask(store, taskID string) (*Completion, error) {
	path := filepath.Join(store, "completed", taskID+".json")
	bytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var completion Completion
	if err := json.Unmarshal(bytes, &completion); err != nil {
		return nil, err
	}
	return &completion, nil
}

func readClaim(path string) (Claim, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Claim{}, err
	}
	var claim Claim
	if err := json.Unmarshal(bytes, &claim); err != nil {
		return Claim{}, err
	}
	if claim.Schema != 1 || claim.TaskID == "" || claim.Owner == "" || claim.Worktree == "" {
		return Claim{}, errors.New("invalid claim record")
	}
	return claim, nil
}

func withCoordinationLock(directory string, fn func(store string) error) error {
	store, err := CoordinationRoot(directory)
	if err != nil {
		return err
	}
	lock := filepath.Join(store, "locks", "coordination.lock")
	if err := os.MkdirAll(filepath.Dir(lock), 0o700); err != nil {
		return err
	}
	deadline := time.Now().Add(lockWait)
	for {
		err := os.Mkdir(lock, 0o700)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("coordination store is busy; retry shortly, or after confirming no command is running use continuity recover-lock --force")
		}
		time.Sleep(15 * time.Millisecond)
	}
	defer os.Remove(lock)
	return fn(store)
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".continuity-*")
	if err != nil {
		return err
	}
	temp := file.Name()
	defer os.Remove(temp)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func normalizeScopes(scope []string) ([]string, error) {
	if len(scope) == 0 {
		return []string{"."}, nil
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(scope))
	for _, item := range scope {
		item = strings.TrimSpace(item)
		if item == "" || item == "none" {
			// Silently widening to "." here would let one typo in a scope list
			// claim the whole repository and reject every other worker.
			return nil, fmt.Errorf("scope %q is not a path; pass repository-relative paths, or \".\" to deliberately claim the whole repository", item)
		}
		if filepath.IsAbs(item) {
			return nil, fmt.Errorf("scope %q must be repository-relative", item)
		}
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean == ".." || strings.HasPrefix(clean, "../") {
			return nil, fmt.Errorf("scope %q escapes the repository", item)
		}
		if !seen[clean] {
			seen[clean] = true
			result = append(result, clean)
		}
	}
	sort.Strings(result)
	return result, nil
}

func validCoordinationTaskID(value string) bool {
	if value == "" {
		return false
	}
	for index, runeValue := range value {
		if (runeValue >= 'A' && runeValue <= 'Z') || (runeValue >= 'a' && runeValue <= 'z') {
			continue
		}
		if index > 0 && ((runeValue >= '0' && runeValue <= '9') || runeValue == '-' || runeValue == '_') {
			continue
		}
		return false
	}
	return true
}

func singleLine(value string) bool {
	return strings.TrimSpace(value) != "" && !strings.ContainsAny(value, "\r\n")
}

func singleLineOrEmpty(value string) bool {
	return strings.TrimSpace(value) == "" || singleLine(value)
}

func scopesOverlap(left, right []string) bool {
	for _, a := range left {
		for _, b := range right {
			if a == "." || b == "." || a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/") {
				return true
			}
		}
	}
	return false
}

func canonicalPath(path string) (string, error) {
	if path == "" {
		return "", errors.New("worktree path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved), nil
	}
	return filepath.Clean(abs), nil
}

func samePath(left, right string) bool {
	leftPath, leftErr := canonicalPath(left)
	rightPath, rightErr := canonicalPath(right)
	return leftErr == nil && rightErr == nil && leftPath == rightPath
}

func randomNonce() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
