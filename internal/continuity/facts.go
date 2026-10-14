package continuity

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	factCardPattern = regexp.MustCompile(`^- \[(fresh|stale)\] (F-[A-Za-z0-9_-]+) \| source: ([^|]+) \| paths: ([^|]+) \| verified: ([^|]+) \| evidence: ([^|]+) \| summary: (.+)$`)
	factIDPattern   = regexp.MustCompile(`^F-[A-Za-z0-9][A-Za-z0-9_-]*$`)
)

// FactCard is a compact, source-linked claim that can age automatically when
// its declared evidence paths change. Its status expresses review freshness;
// it never silently rewrites the underlying engineering decision.
type FactCard struct {
	Status     string
	ID         string
	Source     string
	Paths      []string
	VerifiedAt time.Time
	Evidence   string
	Summary    string
}

func (s *State) FactCards() ([]FactCard, error) {
	return ParseFactCards(s.Section("Fact Cards"))
}

func (s *State) ReplaceFactCards(cards []FactCard) error {
	seen := map[string]bool{}
	for _, card := range cards {
		if err := validateFactCard(card); err != nil {
			return err
		}
		if seen[card.ID] {
			return fmt.Errorf("duplicate fact ID %s", card.ID)
		}
		seen[card.ID] = true
	}
	return s.ReplaceSection("Fact Cards", RenderFactCards(cards))
}

func ParseFactCards(content string) ([]FactCard, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, nil
	}
	seen := map[string]bool{}
	cards := []FactCard{}
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		match := factCardPattern.FindStringSubmatch(line)
		if len(match) == 0 {
			return nil, errors.New("Fact Cards must use '- [fresh] F-001 | source: ... | paths: ... | verified: RFC3339 | evidence: ... | summary: ...'")
		}
		if seen[match[2]] {
			return nil, fmt.Errorf("duplicate fact ID %s", match[2])
		}
		paths, err := parseCapsulePaths(match[4])
		if err != nil {
			return nil, fmt.Errorf("fact %s paths: %w", match[2], err)
		}
		verified, err := time.Parse(time.RFC3339, strings.TrimSpace(match[5]))
		if err != nil {
			return nil, fmt.Errorf("fact %s verified: %w", match[2], err)
		}
		card := FactCard{
			Status:     match[1],
			ID:         match[2],
			Source:     strings.TrimSpace(match[3]),
			Paths:      paths,
			VerifiedAt: verified,
			Evidence:   strings.TrimSpace(match[6]),
			Summary:    strings.TrimSpace(match[7]),
		}
		if err := validateFactCard(card); err != nil {
			return nil, err
		}
		seen[card.ID] = true
		cards = append(cards, card)
	}
	return cards, nil
}

func RenderFactCards(cards []FactCard) string {
	lines := make([]string, 0, len(cards))
	for _, card := range cards {
		lines = append(lines, RenderFactCard(card))
	}
	return strings.Join(lines, "\n")
}

func RenderFactCard(card FactCard) string {
	return fmt.Sprintf("- [%s] %s | source: %s | paths: %s | verified: %s | evidence: %s | summary: %s", card.Status, card.ID, card.Source, strings.Join(card.Paths, ", "), card.VerifiedAt.UTC().Format(time.RFC3339), card.Evidence, card.Summary)
}

func validateFactCard(card FactCard) error {
	if card.Status != "fresh" && card.Status != "stale" {
		return fmt.Errorf("fact %s has unsupported status %q", card.ID, card.Status)
	}
	if !factIDPattern.MatchString(card.ID) {
		return fmt.Errorf("fact ID %q must match F- followed by letters, digits, underscores, or hyphens", card.ID)
	}
	if err := validateCardPaths("fact "+card.ID, card.Paths); err != nil {
		return err
	}
	if card.VerifiedAt.IsZero() {
		return fmt.Errorf("fact %s requires verification time", card.ID)
	}
	return validateCompactFields("fact "+card.ID, card.Source, card.Evidence, card.Summary)
}

// AgeFactCards marks only currently-fresh facts stale when their declared
// source paths overlap changed paths. It returns the newly stale cards so a
// caller can make the required review visible without scanning state prose.
func (s *State) AgeFactCards(changedPaths []string) ([]FactCard, error) {
	cards, err := s.FactCards()
	if err != nil || len(cards) == 0 {
		return nil, err
	}
	changedPaths = normalizeCardPaths(changedPaths)
	if len(changedPaths) == 0 {
		return nil, nil
	}
	aged := []FactCard{}
	for index := range cards {
		if cards[index].Status != "fresh" || !anyPathsOverlap(cards[index].Paths, changedPaths) {
			continue
		}
		cards[index].Status = "stale"
		aged = append(aged, cards[index])
	}
	if len(aged) == 0 {
		return nil, nil
	}
	if err := s.ReplaceFactCards(cards); err != nil {
		return nil, err
	}
	return aged, nil
}

func FactCardsAffectedByPaths(cards []FactCard, paths []string) []FactCard {
	paths = normalizeCardPaths(paths)
	affected := []FactCard{}
	for _, card := range cards {
		if anyPathsOverlap(card.Paths, paths) {
			affected = append(affected, card)
		}
	}
	return affected
}

func normalizeCardPaths(paths []string) []string {
	unique := map[string]bool{}
	normalized := []string{}
	for _, value := range paths {
		value = strings.TrimSpace(value)
		if value == "" || filepath.IsAbs(value) {
			continue
		}
		value = filepath.ToSlash(filepath.Clean(value))
		if value == ".." || strings.HasPrefix(value, "../") || unique[value] {
			continue
		}
		unique[value] = true
		normalized = append(normalized, value)
	}
	sort.Strings(normalized)
	return normalized
}

// validateCardPaths makes direct programmatic writes just as safe as parsed
// Markdown. A list element is exactly one repository-relative path because
// Render* joins list elements with commas.
func validateCardPaths(label string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("%s requires at least one path", label)
	}
	seen := map[string]bool{}
	for _, path := range paths {
		if strings.Contains(path, ",") {
			return fmt.Errorf("%s path %q must be one path, not a comma-separated list", label, path)
		}
		parsed, err := parseCapsulePaths(path)
		if err != nil || len(parsed) != 1 {
			if err != nil {
				return fmt.Errorf("%s path %q: %w", label, path, err)
			}
			return fmt.Errorf("%s path %q is invalid", label, path)
		}
		if seen[parsed[0]] {
			return fmt.Errorf("%s repeats path %q", label, parsed[0])
		}
		seen[parsed[0]] = true
	}
	return nil
}

func anyPathsOverlap(left, right []string) bool {
	for _, first := range left {
		for _, second := range right {
			if cardPathsOverlap(first, second) {
				return true
			}
		}
	}
	return false
}

func cardPathsOverlap(left, right string) bool {
	left = filepath.ToSlash(filepath.Clean(left))
	right = filepath.ToSlash(filepath.Clean(right))
	return left == "." || right == "." || left == right || strings.HasPrefix(left, right+"/") || strings.HasPrefix(right, left+"/")
}

func validateCompactFields(label string, fields ...string) error {
	for _, value := range fields {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n|") {
			return fmt.Errorf("%s fields must be non-empty single-line text without |", label)
		}
	}
	return nil
}
