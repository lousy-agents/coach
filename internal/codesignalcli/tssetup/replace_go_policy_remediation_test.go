package tssetup

import (
	"strings"
	"testing"
)

// The line for a Go policy that exists and cannot be used names the authoring command with the candidate's path, and
// a move whose paths are quoted for a shell and which stages the policy as a literal pathspec, so that a glob
// character in the name matches that file and no other.
func TestTheLineForAGoPolicyToReplaceNamesTheCandidateAndAMoveThatWorksFromAnyDirectory(t *testing.T) {
	line := ReplaceGoPolicyRemediation("cfg/a[1]*.json", "cfg/a[1]*.candidate.json")

	for _, want := range []string{
		"--suggest-project-config --output 'cfg/a[1]*.candidate.json'",
		`top="$(git rev-parse --show-toplevel)" && mv -f -- "$top"/'cfg/a[1]*.candidate.json' "$top"/'cfg/a[1]*.json'`,
		`&& git -C "$top" add -- ':(literal)cfg/a[1]*.json'`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q; want it to contain %q", line, want)
		}
	}
	if !strings.HasSuffix(line, `add -- ':(literal)cfg/a[1]*.json'`) {
		t.Errorf("line %q; want the move as the tail, so that it is pasted whole", line)
	}
}

// A command that names another path would write somewhere else, so a path that cannot be shown gets no line.
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
