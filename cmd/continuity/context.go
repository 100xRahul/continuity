package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"continuity/internal/continuity"
)

// sessionStartContextLimit bounds the restart packet. Claude Code accepts
// 10,000 characters of hook context and Codex about 2,500 tokens by default.
const sessionStartContextLimit = 6000

func contextCommand(args []string) error {
	flags := flag.NewFlagSet("context", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	repo := flags.String("repo", ".", "repository path")
	scope := flags.String("scope", "", "comma-separated repository paths whose context capsules and decisions come first")
	limit := flags.Int("limit", sessionStartContextLimit, "maximum context bytes")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *limit <= 0 {
		return errors.New("--limit must be positive")
	}
	scopes, err := parseContextScopes(*scope)
	if err != nil {
		return err
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	context, err := compactContext(root, *limit, scopes)
	if err != nil {
		return err
	}
	fmt.Print(context)
	return nil
}

type contextPacketSection struct {
	full     string
	fallback string
}

// compactContext builds the restart packet a new session reads first. Its
// critical block — index freshness, broken records, stale facts, uncommitted
// work, and the user's constraints — always appears whole. The plan's next
// work, the last handoff, decision titles, and capsules follow in that order
// of priority and are condensed or omitted when the byte limit is reached.
func compactContext(root string, limit int, scopes []string) (string, error) {
	if limit <= 0 {
		return "", errors.New("context limit must be positive")
	}
	state, err := loadState(root)
	if err != nil {
		return "", err
	}
	plan, err := loadPlan(root)
	if err != nil {
		return "", err
	}
	if result := plan.Validate(); !result.OK() {
		return "", fmt.Errorf("%s is invalid: %s", continuity.PlanFilename, strings.Join(result.Errors, "; "))
	}
	var critical strings.Builder
	critical.WriteString("Continuity restart context\n")
	fingerprint, err := continuity.Fingerprint(root)
	if err != nil {
		return "", err
	}
	if fingerprint != state.Meta["fingerprint"] {
		critical.WriteString("The project index is stale: code changed after the last continuity sync. continuity reconcile lists the changes; update the index entries they affect, then run continuity sync.\n")
	}
	if snapshot := continuity.Snapshot(root); snapshot.Dirty {
		fmt.Fprintf(&critical, "Uncommitted changes in this checkout: %s\n", listSnippet(snapshot.ChangedFiles, 10))
	}
	decisions, problems, err := continuity.LoadDecisions(root)
	if err != nil {
		return "", fmt.Errorf("load decision records: %w", err)
	}
	if len(problems) > 0 {
		fmt.Fprintf(&critical, "Decision records need repair; continuity check lists them: %s\n", singleLineSnippet(strings.Join(problems, "; "), 300))
	}
	facts, err := state.FactCards()
	if err != nil {
		return "", fmt.Errorf("read Fact Cards: %w", err)
	}
	stale := []string{}
	for _, fact := range facts {
		if fact.Status == "stale" {
			stale = append(stale, fact.ID)
		}
	}
	if len(stale) > 0 {
		fmt.Fprintf(&critical, "Stale facts, unreliable until reviewed and confirmed with continuity fact confirm: %s\n", listSnippet(stale, 10))
	}
	if constraints := state.Bullets("Constraints and Approval Boundaries"); len(constraints) > 0 {
		critical.WriteString("Constraints:\n")
		for _, constraint := range constraints {
			critical.WriteString("- " + singleLineSnippet(constraint, 400) + "\n")
		}
	}
	next, err := nextWorkSection(plan)
	if err != nil {
		return "", err
	}
	sections := []contextPacketSection{next}
	if handoff := strings.TrimSpace(state.Section("Handoff")); handoff != "" {
		sections = append(sections, contextPacketSection{full: "Last handoff:\n" + handoff + "\n", fallback: "Last handoff: " + singleLineSnippet(handoff, 400) + "\n"})
	}
	if section, ok := decisionIndexSection(continuity.CurrentDecisions(decisions), scopes, limit); ok {
		sections = append(sections, section)
	}
	capsules, err := state.ContextCapsules()
	if err != nil {
		return "", fmt.Errorf("read Context Capsules: %w", err)
	}
	if selected := continuity.SelectContextCapsules(capsules, scopes); len(selected) > 0 {
		const capsuleHeader = "Context capsules:\n"
		sections = append(sections, contextPacketSection{full: capsuleHeader, fallback: capsuleHeader})
		for _, capsule := range selected {
			fallback := fmt.Sprintf("Context capsule: %s\nPaths: %s\nSummary: %s\n", singleLineSnippet(capsule.Name, 100), singleLineSnippet(strings.Join(capsule.Paths, ","), 220), singleLineSnippet(capsule.Summary, 320))
			sections = append(sections, contextPacketSection{full: capsule.Render(), fallback: fallback})
		}
	}
	return assembleContextPacket(limit, critical.String(), sections)
}

// nextWorkSection states what the user wants built next: the active
// milestone's unfinished outcomes in plan order.
func nextWorkSection(plan *continuity.Plan) (contextPacketSection, error) {
	milestone, open, err := plan.Next()
	if err != nil {
		return contextPacketSection{}, err
	}
	header := "Intent: " + singleLineSnippet(plan.Intent(), 400) + "\n"
	switch {
	case milestone == nil:
		text := header + "Next: no milestone in PROJECT_PLAN.md is active, so no product work is approved yet; the user chooses the next milestone.\n"
		return contextPacketSection{full: text, fallback: text}, nil
	case len(open) == 0:
		text := header + fmt.Sprintf("Next: every outcome of active milestone %s (%s) is done. It is complete when it meets: %s. The user then closes it and chooses the next milestone.\n", milestone.ID, milestone.Title, milestone.Accepts)
		return contextPacketSection{full: text, fallback: text}, nil
	}
	var full, fallback strings.Builder
	intro := fmt.Sprintf("Next: active milestone %s — %s. Open outcomes, in plan order:\n", milestone.ID, milestone.Title)
	full.WriteString(header + intro)
	fallback.WriteString(header + intro)
	for _, outcome := range open {
		fmt.Fprintf(&full, "- %s %s — accepts: %s\n", outcome.ID, outcome.Title, outcome.Accepts)
		fmt.Fprintf(&fallback, "- %s %s\n", outcome.ID, singleLineSnippet(outcome.Title, 160))
	}
	return contextPacketSection{full: full.String(), fallback: fallback.String()}, nil
}

// decisionIndexSection lists current decisions one line each: those linked to
// the requested scope first, then project-wide ones, then the newest others.
// Titles are the always-loaded part; each record holds the reasons and its
// paths, and the post-tool hook shows path-linked records on use. When the
// whole list does not fit, the fallback keeps what fits in two fifths of the
// packet and says how to reach the rest.
func decisionIndexSection(decisions []continuity.Decision, scopes []string, limit int) (contextPacketSection, bool) {
	if len(decisions) == 0 {
		return contextPacketSection{}, false
	}
	rank := func(decision continuity.Decision) int {
		switch {
		case len(scopes) > 0 && !decision.ProjectWide() && contextPathsOverlap(decision.Paths, scopes):
			return 0
		case decision.ProjectWide():
			return 1
		default:
			return 2
		}
	}
	ordered := append([]continuity.Decision(nil), decisions...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if rank(ordered[i]) != rank(ordered[j]) {
			return rank(ordered[i]) < rank(ordered[j])
		}
		return ordered[i].Date.After(ordered[j].Date)
	})
	const header = "Current decisions in docs/adr (the file holds the reasons):\n"
	lines := make([]string, len(ordered))
	for index, decision := range ordered {
		lines[index] = fmt.Sprintf("- %s → %s\n", singleLineSnippet(decision.Title, 160), decision.File)
	}
	var fallback strings.Builder
	fallback.WriteString(header)
	included := 0
	for _, line := range lines {
		if fallback.Len()+len(line) > limit*2/5-120 {
			break
		}
		fallback.WriteString(line)
		included++
	}
	if included < len(lines) {
		fmt.Fprintf(&fallback, "- %d more current decisions; continuity recall --path <file> lists those linked to a file.\n", len(lines)-included)
	}
	return contextPacketSection{full: header + strings.Join(lines, ""), fallback: fallback.String()}, true
}

// assembleContextPacket keeps the critical block whole and then adds each
// section in full, as its fallback, or not at all, so a byte limit never cuts
// a line in half. It fails rather than truncate the critical block.
func assembleContextPacket(limit int, critical string, sections []contextPacketSection) (string, error) {
	if len(critical) > limit {
		return "", fmt.Errorf("context limit %d is too small for the %d-byte critical block; increase --limit", limit, len(critical))
	}
	const noticeReserve = 112
	sectionLimit := limit
	if limit-len(critical) > noticeReserve {
		sectionLimit -= noticeReserve
	}
	var builder strings.Builder
	builder.WriteString(critical)
	fallbackSuffix := make([]int, len(sections)+1)
	for index := len(sections) - 1; index >= 0; index-- {
		fallbackSuffix[index] = fallbackSuffix[index+1]
		if sections[index].fallback != "" {
			fallbackSuffix[index] += len(sections[index].fallback)
		}
	}
	omitted := 0
	condensed := 0
	for index, section := range sections {
		if section.full == "" {
			continue
		}
		if builder.Len()+len(section.full)+fallbackSuffix[index+1] <= sectionLimit {
			builder.WriteString(section.full)
			continue
		}
		if section.fallback != "" && builder.Len()+len(section.fallback) <= sectionLimit {
			builder.WriteString(section.fallback)
			condensed++
			continue
		}
		omitted++
	}
	if omitted > 0 || condensed > 0 {
		notice := fmt.Sprintf("Context budget: %d section(s) omitted, %d condensed; continuity context --limit shows more.\n", omitted, condensed)
		if builder.Len()+len(notice) <= limit {
			builder.WriteString(notice)
		}
	}
	return builder.String(), nil
}

func listSnippet(items []string, limit int) string {
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return fmt.Sprintf("%s (+%d more)", strings.Join(items[:limit], ", "), len(items)-limit)
}

func contextPathsOverlap(left, right []string) bool {
	for _, first := range left {
		first = filepath.ToSlash(filepath.Clean(first))
		for _, second := range right {
			second = filepath.ToSlash(filepath.Clean(second))
			if first == "." || second == "." || first == second || strings.HasPrefix(first, second+"/") || strings.HasPrefix(second, first+"/") {
				return true
			}
		}
	}
	return false
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
