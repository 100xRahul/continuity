package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"continuity/internal/continuity"
)

const (
	maxActiveChangeCapsules = 3
	maxCapsulePaths         = 8
	maxCapsuleEvidence      = 3
	maxArchivedPaths        = 256
	maxArchivedEvidence     = 64
)

// changeRecord is the bounded handoff artifact derived at integration time.
// It carries factual worker data only: a task contract, Git-backed changed
// paths, validation evidence, and the next available approved work.
type changeRecord struct {
	TaskID      string
	Commit      string
	CompletedAt time.Time
	Intent      string
	Result      string
	Paths       []string
	Evidence    []string
	Next        nextWork
	ArchivePath string
}

type nextWork struct {
	Summary string
	Action  string
}

func deriveChangeRecord(root string, task continuity.Task, completion continuity.Completion, next nextWork) (changeRecord, error) {
	paths, err := continuity.ChangedPathsBetween(root, completion.BaseCommit, completion.Commit)
	if err != nil {
		return changeRecord{}, fmt.Errorf("derive changed paths for %s: %w", completion.TaskID, err)
	}
	if len(paths) == 0 {
		paths = normalizeHistoryPaths(completion.Scope)
	}
	if len(paths) == 0 {
		paths = normalizeHistoryPaths(task.Scope)
	}
	if len(paths) == 0 {
		paths = []string{"."}
	}
	intent := conciseChangeLine(completion.Goal, 280)
	if intent == "" {
		intent = conciseChangeLine(task.Title, 280)
	}
	return changeRecord{
		TaskID:      completion.TaskID,
		Commit:      completion.Commit,
		CompletedAt: completion.CompletedAt,
		Intent:      intent,
		Result:      conciseChangeLine(completion.Summary, 280),
		Paths:       paths,
		Evidence:    compactEvidence(completion.Evidence, maxArchivedEvidence),
		Next:        next,
		ArchivePath: changeArchivePath(completion.TaskID),
	}, nil
}

func selectNextWork(root string, state *continuity.State) (nextWork, error) {
	plan, err := loadOptionalPlan(root)
	if err != nil {
		return nextWork{}, fmt.Errorf("load delivery plan: %w", err)
	}
	if plan != nil {
		if result := plan.Validate(); !result.OK() {
			return nextWork{}, fmt.Errorf("delivery plan is invalid: %s", strings.Join(result.Errors, "; "))
		}
	}
	claims, err := continuity.ListClaims(root)
	if err != nil {
		return nextWork{}, err
	}
	candidates, err := readyCandidates(state, plan, claims)
	if err != nil {
		return nextWork{}, err
	}
	if len(candidates) == 0 {
		if plan != nil && plan.Meta["status"] == "approved" {
			outcome, linked, err := completableActiveOutcome(plan, state)
			if err != nil {
				return nextWork{}, err
			}
			if outcome != nil {
				return nextWork{
					Summary: fmt.Sprintf("%s — %s | all %d linked task(s) are integrated; outcome evidence is ready to record.", outcome.ID, conciseChangeLine(outcome.Title, 220), linked),
					Action:  fmt.Sprintf("Verify the outcome acceptance (%q), then run continuity plan outcome complete --id %s --evidence 'command: pass'. If it is not met, derive only the missing focused technical step inside %s.", outcome.Acceptance, outcome.ID, outcome.ID),
				}, nil
			}
			return nextWork{
				Summary: "No unclaimed autonomous task is ready.",
				Action:  "Run continuity ready. If no task is blocked, inspect the active approved outcome and derive only its next focused technical step from an unclaimed integration checkout.",
			}, nil
		}
		return nextWork{
			Summary: "No unclaimed autonomous task is ready.",
			Action:  "Run continuity ready and wait for user-approved work before changing product scope.",
		}, nil
	}
	candidate := candidates[0]
	scope := "."
	if len(candidate.Task.Scope) > 0 {
		scope = strings.Join(candidate.Task.Scope, ",")
	}
	summary := candidate.Task.ID + " — " + conciseChangeLine(candidate.Task.Title, 220)
	if candidate.Outcome != nil {
		summary += fmt.Sprintf(" | outcome: %s (priority %d), unblocks: %d", candidate.Outcome.ID, candidate.OutcomePriority, candidate.Unblocks)
		return nextWork{
			Summary: summary,
			Action:  fmt.Sprintf("Run continuity context --scope %s and continuity ready, then claim %s with an evidence-based --selection-reason.", scope, candidate.Task.ID),
		}, nil
	}
	if candidate.Unblocks > 0 {
		summary += fmt.Sprintf(" | unblocks: %d", candidate.Unblocks)
	}
	return nextWork{
		Summary: summary,
		Action:  fmt.Sprintf("Run continuity context --scope %s and claim %s with continuity start.", scope, candidate.Task.ID),
	}, nil
}

// completableActiveOutcome finds the highest-priority active outcome whose
// task set is already integrated. It lets the next handoff distinguish
// "record verified outcome completion" from "invent another task".
func completableActiveOutcome(plan *continuity.Plan, state *continuity.State) (*continuity.Outcome, int, error) {
	if plan == nil || plan.Meta["status"] != "approved" {
		return nil, 0, nil
	}
	tasks, err := continuity.ParseTasks(state.Section("Ready Queue"))
	if err != nil {
		return nil, 0, err
	}
	var selected *continuity.Outcome
	selectedCount := 0
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone != plan.Meta["active_milestone"] || outcome.Status == "x" {
			continue
		}
		linked, incomplete := tasksForOutcome(tasks, outcome.ID)
		if len(linked) == 0 || len(incomplete) > 0 {
			continue
		}
		if selected == nil || outcome.Priority < selected.Priority || (outcome.Priority == selected.Priority && outcome.ID < selected.ID) {
			candidate := outcome
			selected = &candidate
			selectedCount = len(linked)
		}
	}
	return selected, selectedCount, nil
}

func upsertChangeCapsule(state *continuity.State, record changeRecord) error {
	capsules, err := state.ContextCapsules()
	if err != nil {
		return err
	}
	name := "Change " + record.TaskID
	kept := make([]continuity.ContextCapsule, 0, len(capsules)+1)
	changes := []continuity.ContextCapsule{}
	for _, capsule := range capsules {
		if capsule.Name == name {
			continue
		}
		if strings.HasPrefix(capsule.Name, "Change T-") {
			changes = append(changes, capsule)
			continue
		}
		kept = append(kept, capsule)
	}
	changes = append(changes, changeCapsule(record))
	if len(changes) > maxActiveChangeCapsules {
		changes = changes[len(changes)-maxActiveChangeCapsules:]
	}
	return state.ReplaceContextCapsules(append(kept, changes...))
}

func changeCapsule(record changeRecord) continuity.ContextCapsule {
	details := []string{
		"Details: Commit: " + record.Commit,
		"Changed paths: " + displayPaths(record.Paths, maxCapsulePaths),
		"Validation: " + displayEvidence(record.Evidence, maxCapsuleEvidence),
		"Next: " + record.Next.Summary,
		"Archive: " + record.ArchivePath,
	}
	return continuity.ContextCapsule{
		Name:    "Change " + record.TaskID,
		Paths:   capsulePaths(record.Paths),
		Summary: "Intent: " + record.Intent + " | Result: " + record.Result,
		Details: strings.Join(details, "\n"),
	}
}

func changeArchivePath(taskID string) string {
	return filepath.ToSlash(filepath.Join(continuity.PlanHistoryDir, "changes", taskID+".md"))
}

func outcomeArchivePath(outcomeID string) string {
	return filepath.ToSlash(filepath.Join(continuity.PlanHistoryDir, "outcomes", outcomeID+".md"))
}

func milestoneArchivePath(milestoneID string) string {
	return filepath.ToSlash(filepath.Join(continuity.PlanHistoryDir, "milestones", milestoneID+".md"))
}

func writeChangeArchive(root string, record changeRecord) error {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Change %s\n\n", record.TaskID)
	fmt.Fprintf(&builder, "- Completed: %s\n", record.CompletedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&builder, "- Commit: `%s`\n", record.Commit)
	fmt.Fprintf(&builder, "- Intent: %s\n", record.Intent)
	fmt.Fprintf(&builder, "- Result: %s\n", record.Result)
	fmt.Fprintf(&builder, "- Next task: %s\n", record.Next.Summary)
	builder.WriteString("\n## Changed paths\n\n")
	for _, path := range limitHistoryPaths(record.Paths, maxArchivedPaths) {
		fmt.Fprintf(&builder, "- `%s`\n", path)
	}
	builder.WriteString("\n## Validation evidence\n\n")
	for _, evidence := range compactEvidence(record.Evidence, maxArchivedEvidence) {
		fmt.Fprintf(&builder, "- %s\n", evidence)
	}
	builder.WriteString("\n## Restart action\n\n")
	builder.WriteString(record.Next.Action)
	builder.WriteByte('\n')
	return writeHistoryFile(root, record.ArchivePath, builder.String())
}

// writeOutcomeArchive retains the evidence that makes an outcome complete
// after its detailed task records are no longer routine startup context.
func writeOutcomeArchive(root string, outcome continuity.Outcome, tasks []continuity.Task, evidence []string, next nextWork) error {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Outcome %s\n\n", outcome.ID)
	fmt.Fprintf(&builder, "- Completed: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&builder, "- Milestone: %s\n", outcome.Milestone)
	fmt.Fprintf(&builder, "- Title: %s\n", outcome.Title)
	fmt.Fprintf(&builder, "- Acceptance: %s\n", outcome.Acceptance)
	fmt.Fprintf(&builder, "- Scope: %s\n", strings.Join(outcome.Scope, ", "))
	fmt.Fprintf(&builder, "- Next task: %s\n", next.Summary)
	builder.WriteString("\n## Integrated tasks\n\n")
	for _, task := range tasks {
		fmt.Fprintf(&builder, "- %s — %s | validates: %s\n", task.ID, task.Title, task.Validation)
	}
	builder.WriteString("\n## Completion evidence\n\n")
	for _, item := range compactEvidence(evidence, maxArchivedEvidence) {
		fmt.Fprintf(&builder, "- %s\n", item)
	}
	builder.WriteString("\n## Restart action\n\n")
	builder.WriteString(next.Action)
	builder.WriteByte('\n')
	return writeHistoryFile(root, outcomeArchivePath(outcome.ID), builder.String())
}

// writeMilestoneArchive snapshots the completed milestone before a later one
// becomes active. It deliberately links to smaller outcome archives rather
// than copying every task and change record into a second large document.
func writeMilestoneArchive(root string, plan *continuity.Plan, milestone continuity.Milestone) error {
	var builder strings.Builder
	fmt.Fprintf(&builder, "# Milestone %s\n\n", milestone.ID)
	fmt.Fprintf(&builder, "- Archived: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&builder, "- Title: %s\n", milestone.Title)
	fmt.Fprintf(&builder, "- Acceptance: %s\n", milestone.Acceptance)
	builder.WriteString("\n## Outcomes\n\n")
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone != milestone.ID {
			continue
		}
		fmt.Fprintf(&builder, "- [%s] %s — %s | accepts: %s | archive: `%s`\n", outcome.Status, outcome.ID, outcome.Title, outcome.Acceptance, outcomeArchivePath(outcome.ID))
	}
	builder.WriteString("\n## Archived plan snapshot\n\n")
	builder.WriteString("```text\n")
	builder.WriteString(continuity.RenderMilestone(milestone))
	builder.WriteByte('\n')
	for _, outcome := range mustOutcomes(plan) {
		if outcome.Milestone == milestone.ID {
			builder.WriteString(continuity.RenderOutcome(outcome))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("```\n")
	return writeHistoryFile(root, milestoneArchivePath(milestone.ID), builder.String())
}

func writeHistoryFile(root, relative, contents string) error {
	relative = filepath.ToSlash(filepath.Clean(relative))
	if relative == "." || relative == ".." || strings.HasPrefix(relative, "../") || filepath.IsAbs(relative) {
		return fmt.Errorf("invalid history path %q", relative)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteString(contents); err != nil {
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
	return os.Rename(temporary, path)
}

// presentHistoryFiles returns only records that already exist. Lifecycle
// receipts include their predecessor archives when available, so an agent
// cannot accidentally commit an outcome or milestone record while leaving its
// newly created change archive uncommitted beside it.
func presentHistoryFiles(root string, candidates ...string) ([]string, error) {
	files := make([]string, 0, len(candidates))
	seen := map[string]bool{}
	for _, candidate := range candidates {
		candidate = filepath.ToSlash(filepath.Clean(candidate))
		if candidate == "." || candidate == ".." || strings.HasPrefix(candidate, "../") || filepath.IsAbs(candidate) || seen[candidate] {
			continue
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(candidate)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		seen[candidate] = true
		files = append(files, candidate)
	}
	return files, nil
}

func capsulePaths(paths []string) []string {
	paths = normalizeHistoryPaths(paths)
	if len(paths) <= maxCapsulePaths {
		return paths
	}
	directories := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		directory := filepath.ToSlash(filepath.Dir(path))
		if directory == "." {
			directory = path
		}
		if !seen[directory] {
			seen[directory] = true
			directories = append(directories, directory)
		}
	}
	sort.Strings(directories)
	if len(directories) > maxCapsulePaths {
		// A wide cross-cutting change should remain discoverable from every
		// affected area. A single project-wide short capsule is more honest
		// than silently dropping the ninth and later directories.
		return []string{"."}
	}
	return directories
}

func normalizeHistoryPaths(paths []string) []string {
	unique := map[string]bool{}
	result := []string{}
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || filepath.IsAbs(path) {
			continue
		}
		path = filepath.ToSlash(filepath.Clean(path))
		if path == ".." || strings.HasPrefix(path, "../") || unique[path] {
			continue
		}
		unique[path] = true
		result = append(result, path)
	}
	sort.Strings(result)
	return result
}

func compactEvidence(evidence []string, maximum int) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, item := range evidence {
		item = conciseChangeLine(item, 320)
		if item != "" && !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
		if maximum > 0 && len(result) >= maximum {
			break
		}
	}
	return result
}

func displayPaths(paths []string, maximum int) string {
	paths = normalizeHistoryPaths(paths)
	if len(paths) == 0 {
		return "none"
	}
	if maximum <= 0 || len(paths) <= maximum {
		return strings.Join(paths, ", ")
	}
	return strings.Join(paths[:maximum], ", ") + fmt.Sprintf(" (+%d more)", len(paths)-maximum)
}

func displayEvidence(evidence []string, maximum int) string {
	evidence = compactEvidence(evidence, maximum)
	if len(evidence) == 0 {
		return "none"
	}
	return strings.Join(evidence, "; ")
}

func limitHistoryPaths(paths []string, maximum int) []string {
	paths = normalizeHistoryPaths(paths)
	if maximum > 0 && len(paths) > maximum {
		return paths[:maximum]
	}
	return paths
}

func conciseChangeLine(value string, limit int) string {
	return boundText(strings.Join(strings.Fields(value), " "), limit)
}
