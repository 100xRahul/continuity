package continuity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const PolicyFilename = "CONTINUITY_POLICY.md"

var policyIDPattern = regexp.MustCompile(`^P-[A-Za-z0-9][A-Za-z0-9_-]*$`)

// Policy is an explicitly approved, repository-local command gate. The
// command is intentionally not interpreted by the durable state; only the
// user's configured shell executes it at the named lifecycle phase.
type Policy struct {
	ID      string
	Scope   []string
	Phase   string
	Command string
	Reason  string
}

// PolicyDocument is compact Markdown so every harness can read and edit the
// same policy set without a plugin, service, or hidden configuration store.
type PolicyDocument struct {
	Path     string
	Policies []Policy
}

func PolicyPath(root string) string { return filepath.Join(root, PolicyFilename) }

func NewPolicyDocument(path string) *PolicyDocument { return &PolicyDocument{Path: path} }

func LoadPolicyDocument(path string) (*PolicyDocument, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	document := &PolicyDocument{Path: path}
	if err := document.parse(string(contents)); err != nil {
		return nil, err
	}
	return document, nil
}

func (d *PolicyDocument) parse(contents string) error {
	contents = strings.TrimSpace(strings.ReplaceAll(contents, "\r\n", "\n"))
	if !strings.HasPrefix(contents, "# Continuity policy checks") {
		return errors.New("CONTINUITY_POLICY.md must begin with '# Continuity policy checks'")
	}
	lines := strings.Split(contents, "\n")
	seenChecks := false
	seen := map[string]bool{}
	policies := []Policy{}
	var current *Policy
	flush := func() error {
		if current == nil {
			return nil
		}
		if err := ValidatePolicy(*current); err != nil {
			return err
		}
		if seen[current.ID] {
			return fmt.Errorf("duplicate policy ID %s", current.ID)
		}
		seen[current.ID] = true
		policies = append(policies, *current)
		current = nil
		return nil
	}
	for _, raw := range lines[1:] {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if line == "## Checks" {
			if seenChecks || current != nil {
				return errors.New("CONTINUITY_POLICY.md must contain one ## Checks section")
			}
			seenChecks = true
			continue
		}
		if strings.HasPrefix(line, "### ") {
			if !seenChecks {
				return errors.New("policy records must appear under ## Checks")
			}
			if err := flush(); err != nil {
				return err
			}
			current = &Policy{ID: strings.TrimSpace(strings.TrimPrefix(line, "### "))}
			continue
		}
		if current == nil {
			return fmt.Errorf("unexpected policy line %q", line)
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			return fmt.Errorf("invalid policy field %q", line)
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "Scope":
			if len(current.Scope) > 0 {
				return fmt.Errorf("policy %s has multiple Scope fields", current.ID)
			}
			scope, err := parseCapsulePaths(value)
			if err != nil {
				return fmt.Errorf("policy %s scope: %w", current.ID, err)
			}
			current.Scope = scope
		case "Phase":
			if current.Phase != "" {
				return fmt.Errorf("policy %s has multiple Phase fields", current.ID)
			}
			current.Phase = value
		case "Command":
			if current.Command != "" {
				return fmt.Errorf("policy %s has multiple Command fields", current.ID)
			}
			current.Command = value
		case "Reason":
			if current.Reason != "" {
				return fmt.Errorf("policy %s has multiple Reason fields", current.ID)
			}
			current.Reason = value
		default:
			return fmt.Errorf("policy %s has unknown field %q", current.ID, key)
		}
	}
	if err := flush(); err != nil {
		return err
	}
	if !seenChecks {
		return errors.New("CONTINUITY_POLICY.md is missing ## Checks")
	}
	d.Policies = policies
	return nil
}

func (d *PolicyDocument) Render() string {
	policies := append([]Policy(nil), d.Policies...)
	sort.Slice(policies, func(left, right int) bool { return policies[left].ID < policies[right].ID })
	var builder strings.Builder
	builder.WriteString("# Continuity policy checks\n\n## Checks\n")
	for _, policy := range policies {
		fmt.Fprintf(&builder, "\n### %s\n", policy.ID)
		fmt.Fprintf(&builder, "Scope: %s\n", strings.Join(policy.Scope, ", "))
		fmt.Fprintf(&builder, "Phase: %s\n", policy.Phase)
		fmt.Fprintf(&builder, "Command: %s\n", policy.Command)
		fmt.Fprintf(&builder, "Reason: %s\n", policy.Reason)
	}
	return builder.String()
}

func (d *PolicyDocument) Save() error {
	directory := filepath.Dir(d.Path)
	file, err := os.CreateTemp(directory, "."+filepath.Base(d.Path)+"-*")
	if err != nil {
		return err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	if err := file.Chmod(0o644); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteString(d.Render()); err != nil {
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
	return os.Rename(temporary, d.Path)
}

func (d *PolicyDocument) Add(policy Policy) error {
	if err := ValidatePolicy(policy); err != nil {
		return err
	}
	for _, existing := range d.Policies {
		if existing.ID == policy.ID {
			return fmt.Errorf("policy %s already exists", policy.ID)
		}
	}
	d.Policies = append(d.Policies, policy)
	return nil
}

func ValidatePolicy(policy Policy) error {
	if !policyIDPattern.MatchString(policy.ID) {
		return fmt.Errorf("policy ID %q must match P- followed by letters, digits, underscores, or hyphens", policy.ID)
	}
	for _, value := range []string{policy.ID, policy.Phase, policy.Command, policy.Reason} {
		if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") {
			return errors.New("policy fields must be non-empty single-line text")
		}
	}
	if policy.Phase != "finalize" {
		return fmt.Errorf("policy %s has unsupported phase %q; expected finalize", policy.ID, policy.Phase)
	}
	if err := validateCardPaths("policy "+policy.ID+" scope", policy.Scope); err != nil {
		return err
	}
	return nil
}

func PoliciesForScopes(policies []Policy, scopes []string, phase string) []Policy {
	selected := []Policy{}
	for _, policy := range policies {
		if policy.Phase == phase && (len(scopes) == 0 || anyPathsOverlap(policy.Scope, scopes)) {
			selected = append(selected, policy)
		}
	}
	sort.Slice(selected, func(left, right int) bool { return selected[left].ID < selected[right].ID })
	return selected
}
