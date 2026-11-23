package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy whose committed copy is invalid and that is gone from disk is authored again, and the line names the one step that
// the scan names too. Nothing in it says that a policy must be committed to be used.
func TestPolicyRequiredLineForAnInvalidCommittedPolicyGoneFromDiskNamesTheStepAndNoCommit(t *testing.T) {
	line := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyAuthor, "abc123", "pol.json")

	if want := "coach codesignal: a project policy that passes --check-project is required before compiler setup (policy_invalid); "; !strings.HasPrefix(line, want) {
		t.Errorf("line %q; want it to start with %q", line, want)
	}
	if !strings.Contains(line, unusablePolicyStep) || !strings.HasSuffix(line, "(author_policy).") {
		t.Errorf("line %q; want the step %q and the next action author_policy", line, unusablePolicyStep)
	}
	if strings.Contains(line, "commit") {
		t.Errorf("line %q; a policy on disk needs no commit", line)
	}
}

// A document on disk that is not valid has no remedy: a new policy would replace the file. The refusal says that, and not that the policy is to be moved aside; the order of its arms decides it.
func TestPolicyRequiredLineForAnInvalidPolicyThatNoNewPolicyFixesSaysSo(t *testing.T) {
	for _, path := range []string{"pol.json", ".lousy-agents/coach.json"} {
		t.Run(path, func(t *testing.T) { requireTheNoRemedyLine(t, path) })
	}
}

func requireTheNoRemedyLine(t *testing.T, path string) {
	t.Helper()
	line := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyNone, "abc123", path)

	const want = "(policy_invalid); a new policy cannot fix it, so resolve the policy path first."
	if !strings.HasSuffix(line, want) || strings.Contains(line, "move it aside") || strings.Contains(line, "()") {
		t.Errorf("line %q; want it to end with %q, and not to send the user to move the policy aside", line, want)
	}
}
