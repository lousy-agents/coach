package invocation

import (
	"strings"
	"testing"
)

// The line for a Go policy to replace writes the candidate beside it, with Coach's own command, and describes the rest in
// words: where the candidate is, which file to compare it with, and that the user replaces the policy with it and commits.
// It prints no command that moves or stages a file. Equality holds the whole line, so nothing rides along with the words.
func TestReplaceGoPolicyNamesTheCandidateAndDescribesTheStepInWords(t *testing.T) {
	line := ReplaceGoPolicy("cfg/my policy.json", "cfg/my policy.candidate.json")

	const want = ` -- writes a candidate to "cfg/my policy.candidate.json" beside the policy, because authoring refuses a path that exists; compare it with "cfg/my policy.json", replace the policy with it, commit, and rerun`
	if line != BatchAuthoring("cfg/my policy.candidate.json")+want {
		t.Errorf("line %q; want Coach's authoring command followed by %q", line, want)
	}
	if !strings.Contains(line, "--suggest-project-config --output 'cfg/my policy.candidate.json'") {
		t.Errorf("line %q; want the authoring command with the candidate quoted for a shell", line)
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
