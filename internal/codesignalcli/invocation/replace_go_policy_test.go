package invocation

import (
	"strings"
	"testing"
)

// The line for a Go policy to replace writes the candidate beside it and gives the move over it, quoted and anchored
// at the top of the repository, as the tail of the line.
func TestReplaceGoPolicyNamesTheCandidateAndAMoveThatWorksFromAnyDirectory(t *testing.T) {
	line := ReplaceGoPolicy("cfg/my policy.json", "cfg/my policy.candidate.json")

	for _, want := range []string{
		"--suggest-project-config --output 'cfg/my policy.candidate.json'",
		`top="$(git rev-parse --show-toplevel)" && mv -f -- "$top"/'cfg/my policy.candidate.json' "$top"/'cfg/my policy.json'`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q; want it to contain %q", line, want)
		}
	}
	if !strings.HasSuffix(line, `&& git -C "$top" add -- ':(literal)cfg/my policy.json'`) {
		t.Errorf("line %q; want the stage of the policy as a literal pathspec at the end", line)
	}
}

func TestReplaceGoPolicyPrintsNothingForAPathThatCannotBeShown(t *testing.T) {
	if line := ReplaceGoPolicy("cfg/p\x1b.json", "cfg/p.candidate.json"); line != "" {
		t.Errorf("line %q for a policy name with an escape; want none", line)
	}
	if line := ReplaceGoPolicy("cfg/p.json", "cfg/p\x1b.candidate.json"); line != "" {
		t.Errorf("line %q for a candidate name with an escape; want none", line)
	}
}
