package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A policy that exists and cannot be used is not "never committed": a new policy cannot be authored at its path while it is
// there, so the line says to fix it, or to move it aside and commit the move, which leaves nothing at the path and so brings
// guided authoring back. It names no command.
func TestPolicyRequiredLineForAPolicyThatExistsAndCannotBeUsedSaysToFixItOrMoveItAside(t *testing.T) {
	line := policyRequiredLine("coach codesignal", projectreadiness.GapPolicyInvalid, projectreadiness.RemedyAuthor, "abc123", "pol.json")

	const want = "; the committed policy cannot be used: fix it, or move it aside and commit the move, then run guided policy authoring to write a new one (author_policy)."
	if !strings.HasSuffix(line, want) {
		t.Errorf("line %q; want it to end with %q", line, want)
	}
	if strings.Contains(line, "authoring first") {
		t.Errorf("line %q; guided authoring is not offered while the unusable policy is at the path", line)
	}
}

// A committed document that is not valid, with work in the file (a fix on disk, staged, or in a conflict), has no remedy: a new policy
// would replace the work. The refusal says that, and not that the policy is to be moved aside; the order of its arms decides it.
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
