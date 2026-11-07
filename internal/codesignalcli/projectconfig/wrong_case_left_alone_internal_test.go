package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

type keptCase struct {
	name      string
	typed     string
	spellings []string
	want      string
}

func assertSpellingKept(t *testing.T, c keptCase) {
	t.Helper()
	t.Run(c.name, func(t *testing.T) {
		if got := keptSpelling(c.spellings, c.typed); got != c.want {
			t.Errorf("keptSpelling(%q, %q) = %q; want %q", c.spellings, c.typed, got, c.want)
		}
	})
}

func TestTheSpellingKeptIsTheOneNeedingTheFewestRenamesAndTheDeepestFirstOne(t *testing.T) {
	for _, c := range []keptCase{
		{"fewer components differ", "a/b/c.json", []string{"A/B/c.json", "a/b/C.json"}, "a/b/C.json"},
		{"the first difference is deeper", "a/b/c.json", []string{"a/B/c.json", "A/b/c.json"}, "a/B/c.json"},
		{"the first difference is deeper though a later one is not", "a/b/c.json", []string{"A/b/C.json", "a/B/C.json"}, "a/B/C.json"},
		{"all else equal, the first in order", "x/y.json", []string{"x/y.JSON", "x/Y.json"}, "x/Y.json"},
	} {
		assertSpellingKept(t, c)
	}
}

type leftAloneCase struct {
	name     string
	revision revisionEntries
}

func assertLeftAlone(t *testing.T, c leftAloneCase) {
	t.Helper()
	t.Run(c.name, func(t *testing.T) {
		if got := wrongCaseOf(c.revision, readPath); got != nil {
			t.Errorf("refusal %v; want none", got)
		}
	})
}

// What the revision commits as the path Coach reads, or in no spelling of it, is no case refusal, whatever else it
// commits beside it: with the path as Coach reads it committed, nothing is renamed, and with nothing like it the
// policy is plainly missing or uncommitted.
func TestAPolicyIsLeftAloneWhenTheRevisionHoldsItAsCoachReadsItOrNothingLikeIt(t *testing.T) {
	for _, c := range []leftAloneCase{
		{"nothing at all", fakeRevision{}.entries},
		{"another policy file", fakeRevision{"": {"project.json"}}.entries},
		{"the path as Coach reads it", fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"coach.json"}}.entries},
		{"the directory without the file", fakeRevision{"": {".lousy-agents"}, ".lousy-agents": {"other.json"}}.entries},
		{"the path as Coach reads it, and another spelling beside it", fakeRevision{"": {".lousy-agents", ".LOUSY-AGENTS"}, ".lousy-agents": {"coach.json"}, ".LOUSY-AGENTS": {"coach.json"}}.entries},
		{"the path as Coach reads it, beside more spellings than are named", fakeRevision{"": {".lousy-agents"}, ".lousy-agents": append(variantsOf("coach.json", maxSpellings+1), "coach.json")}.entries},
	} {
		assertLeftAlone(t, c)
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
