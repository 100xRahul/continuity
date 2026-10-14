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
	source := flags.String("source", "", "decision record, constraint, or code-map source")
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
	return updateFacts(*repo, "docs(facts): add "+*id, func(cards []continuity.FactCard) ([]continuity.FactCard, error) {
		for _, card := range cards {
			if card.ID == *id {
				return nil, fmt.Errorf("fact %s already exists", *id)
			}
		}
		return append(cards, continuity.FactCard{
			Status:     "fresh",
			ID:         *id,
			Source:     *source,
			Paths:      paths,
			VerifiedAt: time.Now().UTC(),
			Evidence:   *evidence,
			Summary:    *summary,
		}), nil
	})
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
	return updateFacts(*repo, "docs(facts): confirm "+*id, func(cards []continuity.FactCard) ([]continuity.FactCard, error) {
		for index := range cards {
			if cards[index].ID == *id {
				cards[index].Status = "fresh"
				cards[index].Evidence = *evidence
				cards[index].VerifiedAt = time.Now().UTC()
				return cards, nil
			}
		}
		return nil, fmt.Errorf("unknown fact %s", *id)
	})
}

// updateFacts applies one fact change to a synchronized index and commits it
// at once, so another session or harness sees the receipt immediately.
func updateFacts(repo, message string, change func([]continuity.FactCard) ([]continuity.FactCard, error)) error {
	root, err := repoRoot(repo)
	if err != nil {
		return err
	}
	if err := continuity.EnsureCommitIdentity(root); err != nil {
		return err
	}
	state, err := loadState(root)
	if err != nil {
		return err
	}
	if err := requireFreshState(root, state); err != nil {
		return err
	}
	cards, err := state.FactCards()
	if err != nil {
		return err
	}
	if cards, err = change(cards); err != nil {
		return err
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
	commit, err := continuity.CommitFiles(root, message, continuity.StateFilename)
	if err != nil {
		return err
	}
	fmt.Printf("%s; committed %s\n", message, commit)
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

func ageFactsBeforeSynchronize(root string, state *continuity.State) ([]continuity.FactCard, error) {
	changed, err := evidenceChangedPathsSinceState(root)
	if err != nil {
		return nil, err
	}
	return state.AgeFactCards(changed)
}

// evidenceChangedPathsSinceState lists committed and uncommitted changes since
// the last state commit, including files the source fingerprint ignores, so a
// fact anchored to README.md still ages.
func evidenceChangedPathsSinceState(root string) ([]string, error) {
	changed, err := continuity.ChangedPathsSinceState(root)
	if err != nil {
		return nil, err
	}
	changed = append(changed, continuity.Snapshot(root).ChangedFiles...)
	filtered := make([]string, 0, len(changed))
	for _, path := range changed {
		if path != continuity.StateFilename {
			filtered = append(filtered, path)
		}
	}
	return filtered, nil
}
