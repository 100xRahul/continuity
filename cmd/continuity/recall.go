package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"continuity/internal/continuity"
)

// recallCommitLimit bounds how much Git history keyword recall reads; older
// decisions belong in docs/adr, not only in commit messages.
const recallCommitLimit = 1000

func recallCommand(args []string) error {
	flags := flag.NewFlagSet("recall", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	pathValue := flags.String("path", "", "comma-separated files or directories whose linked decisions, pitfalls, facts, and recent commits to show")
	limit := flags.Int("limit", 8, "maximum keyword results")
	if err := flags.Parse(args); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if (*pathValue == "") == (query == "") {
		return errors.New("usage: continuity recall <words> | continuity recall --path <file>[,<file>...]")
	}
	if *limit <= 0 {
		return errors.New("--limit must be positive")
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	if *pathValue != "" {
		paths, err := repositoryPaths(root, strings.Split(*pathValue, ","))
		if err != nil {
			return err
		}
		report, err := pathRecall(root, paths, nil)
		if err != nil {
			return err
		}
		commits, err := continuity.RecentCommitMessages(root, 5, paths...)
		if err != nil {
			return err
		}
		if len(commits) > 0 {
			report += "Recent commits:\n"
			for _, commit := range commits {
				report += fmt.Sprintf("- %s %s %s\n", commit.Hash, commit.Date, singleLineSnippet(commit.Subject, 160))
			}
		}
		if report == "" {
			report = "No decision, pitfall, fact, or commit is linked to " + strings.Join(paths, ", ") + ".\n"
		}
		fmt.Print(report)
		return nil
	}
	documents, err := recallDocuments(root)
	if err != nil {
		return err
	}
	results := continuity.RankDocuments(documents, query, *limit)
	if len(results) == 0 {
		fmt.Println("No recorded decision, pitfall, fact, history, or commit matches these words. Try other words.")
		return nil
	}
	for _, result := range results {
		fmt.Printf("- [%s] %s — %s\n", result.Kind, singleLineSnippet(result.Summary, 200), result.Location)
	}
	return nil
}

// pathRecall renders the current decisions, pitfalls, and facts linked to
// paths. When shown is not nil, records it already contains are skipped and
// newly rendered ones are added, so a hook repeats nothing within a session.
func pathRecall(root string, paths []string, shown map[string]bool) (string, error) {
	decisions, _, err := continuity.LoadDecisions(root)
	if err != nil {
		return "", err
	}
	state, err := loadState(root)
	if err != nil {
		return "", err
	}
	facts, err := state.FactCards()
	if err != nil {
		return "", err
	}
	fresh := func(key string) bool {
		if shown == nil {
			return true
		}
		if shown[key] {
			return false
		}
		shown[key] = true
		return true
	}
	target := strings.Join(paths, ", ")
	var report strings.Builder
	section := func(heading string, lines []string) {
		if len(lines) > 0 {
			fmt.Fprintf(&report, "%s %s:\n%s", heading, target, strings.Join(lines, ""))
		}
	}
	lines := []string{}
	for _, decision := range continuity.DecisionsForPaths(decisions, paths) {
		if fresh("decision:" + decision.File) {
			lines = append(lines, fmt.Sprintf("- %s → %s\n", decision.Title, decision.File))
		}
	}
	section("Decisions recorded for", lines)
	lines = []string{}
	for _, pitfall := range state.PitfallsForPaths(paths) {
		if fresh("pitfall:" + pitfall) {
			lines = append(lines, "- "+singleLineSnippet(pitfall, 400)+"\n")
		}
	}
	section("Known pitfalls for", lines)
	lines = []string{}
	for _, fact := range continuity.FactCardsAffectedByPaths(facts, paths) {
		if fresh("fact:" + fact.ID) {
			lines = append(lines, fmt.Sprintf("- [%s] %s %s\n", fact.Status, fact.ID, singleLineSnippet(fact.Summary, 300)))
		}
	}
	section("Facts for", lines)
	return report.String(), nil
}

// recallDocuments gathers every durable memory record keyword recall ranks:
// decision records, state sections, committed history, and commit messages.
func recallDocuments(root string) ([]continuity.SearchDocument, error) {
	documents := []continuity.SearchDocument{}
	decisions, _, err := continuity.LoadDecisions(root)
	if err != nil {
		return nil, err
	}
	for _, decision := range decisions {
		kind, location := "decision", decision.File
		if !decision.Current() {
			kind, location = "superseded decision", decision.File+" (superseded by "+strings.Join(decision.SupersededBy, ", ")+")"
		}
		documents = append(documents, continuity.SearchDocument{Kind: kind, Location: location, Summary: decision.Title, Text: decision.Body})
	}
	state, err := loadState(root)
	if err != nil {
		return nil, err
	}
	for _, source := range []struct{ kind, section string }{
		{"pitfall", continuity.PitfallsSection},
		{"constraint", "Constraints and Approval Boundaries"},
		{"code map", "Code Map"},
	} {
		for _, bullet := range state.Bullets(source.section) {
			documents = append(documents, continuity.SearchDocument{Kind: source.kind, Location: continuity.StateFilename + " (" + source.section + ")", Summary: bullet, Text: bullet})
		}
	}
	facts, err := state.FactCards()
	if err != nil {
		return nil, err
	}
	for _, fact := range facts {
		documents = append(documents, continuity.SearchDocument{Kind: "fact (" + fact.Status + ")", Location: continuity.StateFilename + " (" + fact.ID + ")", Summary: fact.Summary, Text: fact.Summary + " " + fact.Evidence + " " + strings.Join(fact.Paths, " ")})
	}
	capsules, err := state.ContextCapsules()
	if err != nil {
		return nil, err
	}
	for _, capsule := range capsules {
		documents = append(documents, continuity.SearchDocument{Kind: "capsule", Location: continuity.StateFilename + " (" + capsule.Name + ")", Summary: capsule.Name + ": " + capsule.Summary, Text: capsule.Name + " " + capsule.Summary + " " + capsule.Details})
	}
	history := filepath.Join(root, continuity.PlanHistoryDir)
	if err := filepath.WalkDir(history, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, os.ErrNotExist) && path == history {
				return filepath.SkipDir
			}
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		summary, _, _ := strings.Cut(strings.TrimSpace(string(contents)), "\n")
		documents = append(documents, continuity.SearchDocument{Kind: "history", Location: filepath.ToSlash(relative), Summary: strings.TrimLeft(summary, "# "), Text: string(contents)})
		return nil
	}); err != nil {
		return nil, err
	}
	commits, err := continuity.RecentCommitMessages(root, recallCommitLimit)
	if err != nil {
		return nil, err
	}
	for _, commit := range commits {
		documents = append(documents, continuity.SearchDocument{Kind: "commit", Location: "commit " + commit.Hash + " (" + commit.Date + ")", Summary: commit.Subject, Text: commit.Subject + "\n" + commit.Body})
	}
	return documents, nil
}

func repositoryPaths(root string, values []string) ([]string, error) {
	paths := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, errors.New("path cannot be empty")
		}
		path, inside := repositoryPath(root, value)
		if !inside {
			return nil, fmt.Errorf("path %q is outside the repository", value)
		}
		paths = append(paths, path)
	}
	return paths, nil
}

// repositoryPath returns value as a clean repository-relative path, or false
// when it is outside the repository. Absolute paths are compared after
// resolving symbolic links, because a harness may report a file through
// another spelling of the same directory, such as /tmp and /private/tmp.
func repositoryPath(root, value string) (string, bool) {
	if filepath.IsAbs(value) {
		relative, err := filepath.Rel(resolvedPath(root), resolvedPath(value))
		if err != nil {
			return "", false
		}
		value = relative
	}
	clean := filepath.ToSlash(filepath.Clean(value))
	if clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

// resolvedPath resolves symbolic links in the longest existing prefix of path,
// so a file that a tool just deleted still maps into its repository.
func resolvedPath(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return filepath.Join(resolvedPath(parent), filepath.Base(path))
}
