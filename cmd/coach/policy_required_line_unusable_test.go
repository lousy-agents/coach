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
