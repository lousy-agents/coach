package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestADiscoveredPolicyCommittedInAnotherCaseIsToldWhereToMoveIt(t *testing.T) {
	cases := []struct {
		name     string
		repoPath string
		revision revisionEntries
		want     []string
		avoid    []string
	}{
		{"the directory and the file differ", ".lousy-agents/coach.json", fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"Coach.json"}}.entries,
			[]string{`the policy is committed as ".Lousy-Agents/Coach.json", but Coach reads ".lousy-agents/coach.json", so it is not read`, "rename it with mkdir -p '.lousy-agents' && git mv '.Lousy-Agents/Coach.json' '.lousy-agents/coach.json' and commit the rename"}, nil},
		{"only the directory differs", ".lousy-agents/coach.json", fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"coach.json"}}.entries,
			[]string{`committed as ".Lousy-Agents/coach.json"`, "mkdir -p '.lousy-agents' && git mv '.Lousy-Agents/coach.json' '.lousy-agents/coach.json'"}, nil},
		{"only the file differs", ".lousy-agents/coach.json", fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json"}}.entries,
			[]string{"git mv '.lousy-agents/Coach.json' '.lousy-agents/coach.json'"}, nil},
		{"a policy at the root, which needs no directory", "policy.json", fakeRevision{"": {"Policy.json"}}.entries,
			[]string{"git mv 'Policy.json' 'policy.json'"}, []string{"mkdir"}},
		{"two spellings of it", ".lousy-agents/coach.json", fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"Coach.json", "COACH.JSON"}}.entries,
			[]string{`several spellings, ".lousy-agents/COACH.JSON", ".lousy-agents/Coach.json"`, "keep one of them at that path"}, []string{"git mv"}},
		{"a name that cannot be shown to a terminal", "policy\x1b.json", fakeRevision{"": {"Policy\x1b.json"}}.entries,
			[]string{"move it to that path and commit the rename"}, []string{"git mv", "mkdir"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertWrongCaseAdvice(t, c.revision, c.repoPath, c.want, c.avoid)
		})
	}
}

func assertWrongCaseAdvice(t *testing.T, revision revisionEntries, repoPath string, want, avoid []string) {
	t.Helper()
	verdict := wrongCaseOf(revision, repoPath)

	if verdict == nil {
		t.Fatalf("no refusal for %q; want one", repoPath)
	}
	got := configErrorOf(t, verdict)
	if got.Kind != KindInvalid || string(got.PolicyCause) != "name_case_differs" || got.Remedy != 0 {
		t.Errorf("refusal = kind %v, cause %q, remedy %d; want an invalid policy with the name_case_differs cause and no remedy", got.Kind, got.PolicyCause, got.Remedy)
	}
	assertCaseMismatchAdvice(t, got, want, avoid)
}

func TestADiscoveredPolicyIsLeftAloneWhenTheRevisionHoldsNothingLikeIt(t *testing.T) {
	cases := []struct {
		name     string
		revision revisionEntries
	}{
		{"nothing at all", fakeRevision{}.entries},
		{"another policy file", fakeRevision{"": {"project.json"}}.entries},
		{"the directory without the file", fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"other.json"}}.entries},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNoWrongCase(t, c.revision)
		})
	}
}

func assertNoWrongCase(t *testing.T, revision revisionEntries) {
	t.Helper()
	if got := wrongCaseOf(revision, ".lousy-agents/coach.json"); got != nil {
		t.Errorf("refusal %v; want none, so the policy is plainly missing", got)
	}
}

// Discover runs against a real repository, so the hook cannot be deleted without a failure on any machine.
func TestDiscoverRefusesAPolicyCommittedInAnotherCaseInsteadOfScanningWithoutOne(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, ".Lousy-Agents/Coach.json", "{}")

	selection, err := Discover(repo, "HEAD")

	var found *ConfigError
	if err != nil || !errors.As(selection.Fatal, &found) {
		t.Fatalf("Discover = %+v, %v; want a fatal ConfigError", selection, err)
	}
	if !strings.Contains(found.Message, `committed as ".Lousy-Agents/Coach.json"`) || string(found.PolicyCause) != "name_case_differs" {
		t.Errorf("refusal = %q (%s); want the committed spelling and the name_case_differs cause", found.Message, found.PolicyCause)
	}
}

func TestDiscoverStillFallsBackToTheLegacyPolicyWhenNothingIsInAnotherCase(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "project.json", `{"schema_version":"1","roots":["."]}`)

	selection, err := Discover(repo, "HEAD")

	if err != nil || selection.Fatal != nil || selection.Path != DefaultPath || !selection.Deprecated {
		t.Errorf("Discover = %+v, %v; want the deprecated project.json", selection, err)
	}
}
