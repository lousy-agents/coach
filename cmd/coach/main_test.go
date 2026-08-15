package main

import (
	"strings"
	"testing"
)

// TestValidateSuggestProjectConfigFlagsRejectsUnenumeratedFlag proves
// validateSuggestProjectConfigFlags rejects a flag it was never told about
// by name, since it checks setFlags against the fixed --suggest-
// project-config allowlist ({baseline, output, suggest-project-config})
// rather than enumerating every flag known to conflict. Every real
// codesignal flag today is already covered by the acceptance-level
// "rejected flag combinations" DescribeTable in
// project_config_suggestion_acceptance_test.go; this synthetic flag name
// (which cannot be constructed from the CLI's actual flag.FlagSet without
// adding a new flag, out of this fix's scope) is the only way to exercise
// the allowlist's structural guarantee for a flag that doesn't exist yet.
func TestValidateSuggestProjectConfigFlagsRejectsUnenumeratedFlag(t *testing.T) {
	f := codesignalFlags{baseline: true, suggestProjectConfig: true}
	setFlags := map[string]bool{
		"suggest-project-config": true,
		"baseline":               true,
		"totally-new-flag":       true,
	}

	got := validateSuggestProjectConfigFlags(f, setFlags, nil, 1, 0)

	if got == "" {
		t.Fatalf("validateSuggestProjectConfigFlags returned no error for an unenumerated flag; expected rejection")
	}
	if !strings.Contains(got, "totally-new-flag") {
		t.Errorf("validateSuggestProjectConfigFlags = %q, want a message naming the unenumerated flag", got)
	}
}
