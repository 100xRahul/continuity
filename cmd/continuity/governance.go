package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"continuity/internal/continuity"
)

func factCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: continuity fact add|confirm|status ...")
	}
	switch args[0] {
	case "add":
		return factAddCommand(args[1:])
	case "confirm":
		return factConfirmCommand(args[1:])
	case "status":
		return factStatusCommand(args[1:])
	default:
		return errors.New("usage: continuity fact add|confirm|status ...")
	}
}

func factAddCommand(args []string) error {
	flags := flag.NewFlagSet("fact add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "fact ID")
	source := flags.String("source", "", "durable decision, constraint, or code-map source")
	pathsValue := flags.String("paths", "", "comma-separated source paths")
	evidence := flags.String("evidence", "", "compact review evidence")
	summary := flags.String("summary", "", "compact fact summary")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{
		"--id":       *id,
		"--source":   *source,
		"--paths":    *pathsValue,
		"--evidence": *evidence,
		"--summary":  *summary,
	}); err != nil {
		return err
	}
	paths, err := parseContextScopes(*pathsValue)
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
		state, err := loadState(root)
		if err != nil {
			return err
		}
		cards, err := state.FactCards()
		if err != nil {
			return err
		}
		for _, card := range cards {
			if card.ID == *id {
				return fmt.Errorf("fact %s already exists", *id)
			}
		}
		cards = append(cards, continuity.FactCard{
			Status:     "fresh",
			ID:         *id,
			Source:     *source,
			Paths:      paths,
			VerifiedAt: time.Now().UTC(),
			Evidence:   *evidence,
			Summary:    *summary,
		})
		if err := state.ReplaceFactCards(cards); err != nil {
			return err
		}
		if err := synchronize(state, root); err != nil {
			return err
		}
		if err := state.Save(); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(facts): add "+*id, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("added fact %s and committed %s\n", *id, commit)
	return nil
}

func factConfirmCommand(args []string) error {
	flags := flag.NewFlagSet("fact confirm", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "fact ID")
	evidence := flags.String("evidence", "", "new compact review evidence")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{"--id": *id, "--evidence": *evidence}); err != nil {
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
		state, err := loadState(root)
		if err != nil {
			return err
		}
		cards, err := state.FactCards()
		if err != nil {
			return err
		}
		found := false
		for index := range cards {
			if cards[index].ID != *id {
				continue
			}
			cards[index].Status = "fresh"
			cards[index].Evidence = *evidence
			cards[index].VerifiedAt = time.Now().UTC()
			found = true
		}
		if !found {
			return fmt.Errorf("unknown fact %s", *id)
		}
		if err := state.ReplaceFactCards(cards); err != nil {
			return err
		}
		if err := synchronize(state, root); err != nil {
			return err
		}
		if err := state.Save(); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(facts): confirm "+*id, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("confirmed fact %s and committed %s\n", *id, commit)
	return nil
}

func factStatusCommand(args []string) error {
	root, err := parseRepo(args, "fact status")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	cards, err := state.FactCards()
	if err != nil {
		return err
	}
	if len(cards) == 0 {
		fmt.Println("Fact cards: none")
		return nil
	}
	for _, card := range cards {
		fmt.Printf("- [%s] %s | source: %s | paths: %s | verified: %s\n", card.Status, card.ID, card.Source, strings.Join(card.Paths, ","), card.VerifiedAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func blockerCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: continuity blocker add|resolve|status ...")
	}
	switch args[0] {
	case "add":
		return blockerAddCommand(args[1:])
	case "resolve":
		return blockerResolveCommand(args[1:])
	case "status":
		return blockerStatusCommand(args[1:])
	default:
		return errors.New("usage: continuity blocker add|resolve|status ...")
	}
}

func blockerAddCommand(args []string) error {
	flags := flag.NewFlagSet("blocker add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "blocker ID")
	task := flags.String("task", "none", "affected task ID or none")
	scopeValue := flags.String("scope", "", "comma-separated blocked scope")
	hypothesis := flags.String("hypothesis", "", "falsifiable current blocker hypothesis")
	evidence := flags.String("evidence", "", "compact supporting evidence")
	nextAction := flags.String("next-action", "", "what could produce new evidence or unblock work")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{
		"--id":          *id,
		"--task":        *task,
		"--scope":       *scopeValue,
		"--hypothesis":  *hypothesis,
		"--evidence":    *evidence,
		"--next-action": *nextAction,
	}); err != nil {
		return err
	}
	scope, err := parseContextScopes(*scopeValue)
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
		if err := continuity.EnsureCommitIdentity(root); err != nil {
			return err
		}
		state, err := loadState(root)
		if err != nil {
			return err
		}
		cards, err := state.BlockerCards()
		if err != nil {
			return err
		}
		for _, card := range cards {
			if card.Status == "open" && card.TaskID == *task && card.Hypothesis == *hypothesis && card.Evidence == *evidence {
				return fmt.Errorf("blocker %s already records this task, hypothesis, and evidence; do not retry without new evidence or a new hypothesis", card.ID)
			}
			if card.ID == *id {
				return fmt.Errorf("blocker %s already exists", *id)
			}
		}
		cards = append(cards, continuity.BlockerCard{
			Status:     "open",
			ID:         *id,
			TaskID:     *task,
			Scope:      scope,
			Hypothesis: *hypothesis,
			Evidence:   *evidence,
			NextAction: *nextAction,
			Resolution: "none",
		})
		if err := state.ReplaceBlockerCards(cards); err != nil {
			return err
		}
		if _, err := synchronizeWithAging(state, root); err != nil {
			return err
		}
		if err := state.Save(); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(blocker): record "+*id, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("recorded blocker %s and committed %s\n", *id, commit)
	return nil
}

func blockerResolveCommand(args []string) error {
	flags := flag.NewFlagSet("blocker resolve", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	id := flags.String("id", "", "blocker ID")
	evidence := flags.String("evidence", "", "new evidence that resolves the blocker")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := requireSingleLine(map[string]string{"--id": *id, "--evidence": *evidence}); err != nil {
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
		state, err := loadState(root)
		if err != nil {
			return err
		}
		cards, err := state.BlockerCards()
		if err != nil {
			return err
		}
		found := false
		for index := range cards {
			if cards[index].ID != *id {
				continue
			}
			if cards[index].Status == "resolved" {
				return fmt.Errorf("blocker %s is already resolved", *id)
			}
			cards[index].Status = "resolved"
			cards[index].Resolution = *evidence
			found = true
		}
		if !found {
			return fmt.Errorf("unknown blocker %s", *id)
		}
		if err := state.ReplaceBlockerCards(cards); err != nil {
			return err
		}
		if err := synchronize(state, root); err != nil {
			return err
		}
		if err := state.Save(); err != nil {
			return err
		}
		commit, err = continuity.CommitFiles(root, "docs(blocker): resolve "+*id, continuity.StateFilename)
		return err
	}); err != nil {
		return err
	}
	fmt.Printf("resolved blocker %s and committed %s\n", *id, commit)
	return nil
}

func blockerStatusCommand(args []string) error {
	root, err := parseRepo(args, "blocker status")
	if err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	cards, err := state.BlockerCards()
	if err != nil {
		return err
	}
	if len(cards) == 0 {
		fmt.Println("Blocker cards: none")
		return nil
	}
	for _, card := range cards {
		fmt.Printf("- [%s] %s | task: %s | scope: %s | next: %s\n", card.Status, card.ID, card.TaskID, strings.Join(card.Scope, ","), card.NextAction)
	}
	return nil
}

// ageFactsBeforeSynchronize is the only automatic fact mutation: it marks
// affected cards stale when their declared sources changed. It never rewrites
// their content or marks a fact fresh without a human/agent review receipt.
func ageFactsBeforeSynchronize(root string, state *continuity.State) ([]continuity.FactCard, error) {
	changed, err := evidenceChangedPathsSinceState(root)
	if err != nil {
		return nil, err
	}
	return state.AgeFactCards(changed)
}

// evidenceChangedPathsSinceState includes paths whose change must invalidate
// source-linked facts. PROJECT_STATE.md itself is the index and would make a
// fact age every time its own review receipt is written, so it is excluded.
func evidenceChangedPathsSinceState(root string) ([]string, error) {
	changed, err := continuity.ChangedPathsSinceState(root)
	if err != nil {
		return nil, err
	}
	for _, path := range continuity.Snapshot(root).ChangedFiles {
		changed = append(changed, path)
	}
	filtered := make([]string, 0, len(changed))
	for _, path := range changed {
		if path == continuity.StateFilename {
			continue
		}
		filtered = append(filtered, path)
	}
	return filtered, nil
}
