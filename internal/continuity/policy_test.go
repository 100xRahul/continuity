package continuity

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyDocumentRoundTripAndScopedSelection(t *testing.T) {
	document := NewPolicyDocument(filepath.Join(t.TempDir(), PolicyFilename))
	if err := document.Add(Policy{ID: "P-001", Scope: []string{"internal/continuity"}, Phase: "finalize", Command: "go test ./internal/continuity | tail -n 1", Reason: "State changes need focused validation."}); err != nil {
		t.Fatal(err)
	}
	if err := document.Add(Policy{ID: "P-002", Scope: []string{"cmd/continuity"}, Phase: "finalize", Command: "go test ./cmd/continuity", Reason: "CLI changes need command validation."}); err != nil {
		t.Fatal(err)
	}
	if err := document.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadPolicyDocument(document.Path)
	if err != nil {
		t.Fatal(err)
	}
	selected := PoliciesForScopes(loaded.Policies, []string{"internal/continuity/state.go"}, "finalize")
	if len(selected) != 1 || selected[0].ID != "P-001" || selected[0].Command != "go test ./internal/continuity | tail -n 1" {
		t.Fatalf("selected policies = %#v", selected)
	}
}

func TestPolicyValidationRejectsMalformedIdentifier(t *testing.T) {
	err := ValidatePolicy(Policy{ID: "P-?", Scope: []string{"internal"}, Phase: "finalize", Command: "go test ./...", Reason: "Test malformed policy IDs."})
	if err == nil || !strings.Contains(err.Error(), "policy ID") {
		t.Fatalf("invalid policy ID was accepted: %v", err)
	}
}

func TestPolicyValidationRejectsAmbiguousDirectScope(t *testing.T) {
	err := ValidatePolicy(Policy{ID: "P-001", Scope: []string{"internal,cmd"}, Phase: "finalize", Command: "go test ./...", Reason: "Test direct scope validation."})
	if err == nil || !strings.Contains(err.Error(), "comma-separated") {
		t.Fatalf("ambiguous direct policy scope was accepted: %v", err)
	}
}
