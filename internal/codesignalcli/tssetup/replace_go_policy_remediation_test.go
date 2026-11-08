package tssetup

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/invocation"
)

// The line for a Go policy that exists and cannot be used names the authoring command with the candidate's path
// quoted for a shell, and describes the replacement in words. A glob character in a name is shown as it is: the words
// name the file, and no command is printed that a shell or a pathspec could read it in.
func TestTheLineForAGoPolicyToReplaceNamesTheCandidateAndDescribesTheStepInWords(t *testing.T) {
	line := ReplaceGoPolicyRemediation("cfg/a[1]*.json", "cfg/a[1]*.candidate.json")

	const want = ` -- writes a candidate to "cfg/a[1]*.candidate.json" beside the policy, because authoring refuses a path that exists; compare it with "cfg/a[1]*.json", replace the policy with it, commit, and rerun`
	if line != invocation.BatchAuthoring("cfg/a[1]*.candidate.json")+want {
		t.Errorf("line %q; want Coach's authoring command followed by %q", line, want)
	}
	if !strings.Contains(line, "--suggest-project-config --output 'cfg/a[1]*.candidate.json'") {
		t.Errorf("line %q; want the authoring command with the candidate quoted for a shell", line)
	}
}

// A path that cannot be shown is not named, and a command that names another path would write somewhere else, so such a path gets no line.
func TestNoLineIsPrintedForAGoPolicyWhoseNameCannotBeShown(t *testing.T) {
	cases := map[string][2]string{
		"an escape in the policy":           {"cfg/p\x1b.json", "cfg/p.candidate.json"},
		"an escape in the candidate":        {"cfg/p.json", "cfg/p\x1b.candidate.json"},
		"a path that leaves the repository": {"../p.json", "../p.candidate.json"},
	}
	for name, paths := range cases {
		t.Run(name, func(t *testing.T) { requireNoLine(t, paths[0], paths[1]) })
	}
}

func requireNoLine(t *testing.T, policyPath, candidate string) {
	t.Helper()
	if line := ReplaceGoPolicyRemediation(policyPath, candidate); line != "" {
		t.Errorf("line %q; want none", line)
	}
}
