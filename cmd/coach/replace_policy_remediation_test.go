package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

func touch(t *testing.T, root, relative string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Authoring is create-only, so the candidate goes to the first name beside the policy that nothing is at.
func TestTheCandidateIsWrittenToTheFirstFreeNameBesideThePolicy(t *testing.T) {
	cases := map[string]struct {
		taken  []string
		policy string
		want   string
	}{
		"nothing is taken":               {nil, "cfg/p.json", "cfg/p.candidate.json"},
		"the first name is taken":        {[]string{"cfg/p.candidate.json"}, "cfg/p.json", "cfg/p.candidate-2.json"},
		"the first two names are taken":  {[]string{"cfg/p.candidate.json", "cfg/p.candidate-2.json"}, "cfg/p.json", "cfg/p.candidate-3.json"},
		"a policy at the root":           {nil, "project.json", "project.candidate.json"},
		"a name without the extension":   {nil, "policy", "policy.candidate.json"},
		"a name taken in another folder": {[]string{"other/p.candidate.json"}, "cfg/p.json", "cfg/p.candidate.json"},
		"the canonical path":             {nil, ".lousy-agents/coach.json", ".lousy-agents/coach.candidate.json"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheFreeCandidate(t, c.taken, c.policy, c.want) })
	}
}

func requireTheFreeCandidate(t *testing.T, taken []string, policy, want string) {
	t.Helper()
	root := t.TempDir()
	for _, name := range taken {
		touch(t, root, name)
	}

	got, found := freeCandidateBeside(root, policy)

	if !found || got != want {
		t.Errorf("freeCandidateBeside = %q, %v; want %q", got, found, want)
	}
}

func TestNoCandidateIsNamedWhenEveryNameBesideThePolicyIsTaken(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "p.candidate.json")
	for attempt := 2; attempt <= maxCandidateSuffix; attempt++ {
		touch(t, root, fmt.Sprintf("p.candidate-%d.json", attempt))
	}

	if got, found := freeCandidateBeside(root, "p.json"); found {
		t.Errorf("freeCandidateBeside = %q, true; want none, since every name is taken", got)
	}
}

// A path that cannot be checked is not free: a file where a directory should be answers with an error that is not
// "does not exist", and a command for it would be refused.
func TestACandidatePathThatCannotBeCheckedIsNotFree(t *testing.T) {
	root := t.TempDir()
	touch(t, root, "cfg")

	if got, found := freeCandidateBeside(root, "cfg/p.json"); found {
		t.Errorf("freeCandidateBeside = %q, true; want none, since a file stands where cfg should be", got)
	}
}

// Outside a repository there is no root to look for a free name in, and no line is printed.
func TestNoGoReplacementLineIsPrintedOutsideARepository(t *testing.T) {
	if line := goReplaceRemediation(t.TempDir(), policyref.Named("cfg/p.json")); line != "" {
		t.Errorf("goReplaceRemediation = %q; want none outside a repository", line)
	}
}

func TestTheGoReplacementLineIsForTheDefaultPolicyWhenNoneIsNamed(t *testing.T) {
	repo := initializedRepository(t)

	line := goReplaceRemediation(repo, policyref.Ref{})

	const words = ` -- writes a candidate to ".lousy-agents/coach.candidate.json" beside the policy, because authoring refuses a path that exists; compare it with ".lousy-agents/coach.json", replace the policy with it, commit, and rerun`
	if !strings.Contains(line, "--output .lousy-agents/coach.candidate.json") || !strings.HasSuffix(line, words) {
		t.Errorf("goReplaceRemediation = %q; want the candidate beside the default policy and the words that say to replace the policy with it", line)
	}
}
