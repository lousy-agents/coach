package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy whose committed copy is invalid and that is gone from disk is authored again, and the line names the one step that
// the scan names too. Nothing in it says that a policy must be committed to be used.
func TestPolicyRequiredLineForAnInvalidCommittedPolicyGoneFromDiskNamesTheStepAndNoCommit(t *testing.T) {
	line := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyAuthor, "", "abc123", "pol.json")

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

// A regular file on disk that cannot be used has no remedy: a new policy would replace the file. The refusal says to fix the
// file, the sentence the scan prints, and not that the policy is to be moved aside.
func TestPolicyRequiredLineForAFileOnDiskThatCannotBeUsedSaysToFixTheFile(t *testing.T) {
	for _, cause := range []projectreadiness.PolicyCause{"", projectreadiness.CauseUndetermined, projectreadiness.CauseUnreadableOnDisk} {
		t.Run(string(cause), func(t *testing.T) {
			line := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyNone, cause, "abc123", "pol.json")

			const want = "(policy_invalid); " + unusablePolicyStep + "."
			if !strings.HasSuffix(line, want) || strings.Contains(line, "move it aside") || strings.Contains(line, "resolve the policy path first") {
				t.Errorf("line %q; want it to end with %q and not to send the user to the policy path", line, want)
			}
		})
	}
}

// A policy path that holds a directory or a link is resolved at the path: fixing a file does not apply.
func TestPolicyRequiredLineForAPathThatHoldsNoFileSaysToResolveThePathFirst(t *testing.T) {
	for _, cause := range []projectreadiness.PolicyCause{projectreadiness.CauseNotARegularFile, projectreadiness.CauseSymbolicLink, projectreadiness.CauseParentNotDirectory, projectreadiness.CauseDirectory} {
		t.Run(string(cause), func(t *testing.T) {
			line := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyNone, cause, "abc123", "pol.json")

			const want = "(policy_invalid); a new policy cannot fix it, so resolve the policy path first."
			if !strings.HasSuffix(line, want) || strings.Contains(line, unusablePolicyStep) {
				t.Errorf("line %q; want it to end with %q", line, want)
			}
		})
	}
}

// A gap that is not policy_invalid keeps the path advice whatever its cause word is.
func TestPolicyRequiredLineForAMissingPolicyNoRemedyFixesSaysToResolveThePathFirst(t *testing.T) {
	line := policyRequiredLine("coach codesignal", "policy_missing", projectreadiness.RemedyNone, projectreadiness.CauseUndetermined, "abc123", "pol.json")

	if want := "(policy_missing); a new policy cannot fix it, so resolve the policy path first."; !strings.HasSuffix(line, want) {
		t.Errorf("line %q; want it to end with %q", line, want)
	}
}
