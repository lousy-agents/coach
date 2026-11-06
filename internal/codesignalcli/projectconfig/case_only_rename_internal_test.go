package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

func TestAnUncommittedPolicyTheRevisionHoldsInAnotherCaseIsToldHowToRecordTheRename(t *testing.T) {
	policies := fakeRevision{"": {"policies"}, "policies": {"p.json"}}.entries
	cases := []struct {
		name     string
		repoPath string
		revision revisionEntries
		want     []string
		avoid    []string
	}{
		{"a file", "policies/P.json", policies,
			[]string{`the analyzed revision holds the path as "policies/p.json"`, "git mv 'policies/p.json' 'policies/P.json' and commit it, or rename the file back and pass --project-config policies/p.json"}, []string{"commit the file"}},
		{"a directory", "Policies/p.json", policies,
			[]string{`holds the path as "policies/p.json"`, `rename "Policies" back to "policies" and pass --project-config policies/p.json`}, []string{"git mv"}},
		{"a directory above a file that differs too", "Policies/P.json", policies,
			[]string{`rename "Policies" back to "policies"`}, []string{"git mv"}},
		{"a directory below the root", "a/B/c.json", fakeRevision{"": {"a"}, "a": {"b"}, "a/b": {"c.json"}}.entries,
			[]string{`rename "a/B" back to "a/b" and pass --project-config a/b/c.json`}, []string{"git mv"}},
		{"a file whose name cannot be shown to a terminal", "policies/P\x1b.json", fakeRevision{"": {"policies"}, "policies": {"p\x1b.json"}}.entries,
			[]string{"rename"}, []string{"git mv", "and pass --project-config"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRenameAdvice(t, c.revision, c.repoPath, c.want, c.avoid)
		})
	}
}

func assertRenameAdvice(t *testing.T, revision revisionEntries, repoPath string, want, avoid []string) {
	t.Helper()
	got := caseOnlyRename(revision, repoPath)

	if got == nil {
		t.Fatalf("caseOnlyRename(%q) = nil; want a refusal", repoPath)
	}
	assertCaseMismatchAdvice(t, got, want, avoid)
}

func TestAnUncommittedPolicyIsLeftAloneWhenTheRevisionHoldsNoOtherCaseOfIt(t *testing.T) {
	cases := []struct {
		name     string
		repoPath string
		revision revisionEntries
	}{
		{"the revision holds the path as typed", "policies/p.json", fakeRevision{"": {"policies"}, "policies": {"p.json"}}.entries},
		{"the revision holds no such name", "policies/q.json", fakeRevision{"": {"policies"}, "policies": {"p.json"}}.entries},
		{"the revision holds two other cases of it", "policies/P.json", fakeRevision{"": {"policies"}, "policies": {"p.json", "P.JSON"}}.entries},
		{"the revision cannot be asked", "policies/P.json", failingRevision},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPlainlyUncommitted(t, c.revision, c.repoPath)
		})
	}
}

func assertPlainlyUncommitted(t *testing.T, revision revisionEntries, repoPath string) {
	t.Helper()
	if got := caseOnlyRename(revision, repoPath); got != nil {
		t.Errorf("caseOnlyRename(%q) = %q; want nil, so the policy is plainly uncommitted", repoPath, got.Message)
	}
}

func TestTheCaseOnlyRenameIsOnlyAnIssueWhereGitIgnoresCase(t *testing.T) {
	revision := fakeRevision{"": {"policies"}, "policies": {"p.json"}}.entries
	cases := []struct {
		name       string
		ignoreCase string
		err        error
		want       bool
	}{
		{"core.ignorecase is true", "true\n", nil, true},
		{"core.ignorecase is false", "false\n", nil, false},
		{"git cannot say", "", errors.New("exit status 1"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRefusedOnlyWhereGitIgnoresCase(t, c.ignoreCase, c.err, revision, c.want)
		})
	}
}

func assertRefusedOnlyWhereGitIgnoresCase(t *testing.T, ignoreCase string, err error, revision revisionEntries, want bool) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return []byte(ignoreCase), err }

	got := caseOnlyRenameRefusalWith(run, "repo", revision, "policies/P.json")

	if (got != nil) != want {
		t.Errorf("refusal = %v; want a refusal = %v", got, want)
	}
}

func TestTheGitMoveCommandIsShownOnlyForNamesThatCanBePrintedAndPasted(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
		want     string
	}{
		{"plain names", "policies/p.json", "policies/P.json", "git mv 'policies/p.json' 'policies/P.json'"},
		{"a name with an apostrophe", "it's.json", "It's.json", `git mv 'it'\''s.json' 'It'\''s.json'`},
		{"a control byte", "a\x1b.json", "A\x1b.json", ""},
		{"a path outside the repository", "../p.json", "../P.json", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertGitMoveCommand(t, c.from, c.to, c.want)
		})
	}
}

func assertGitMoveCommand(t *testing.T, from, to, want string) {
	t.Helper()
	got, ok := gitMoveCommand(from, to)

	if got != want || ok != (want != "") {
		t.Errorf("gitMoveCommand(%q, %q) = %q, %v; want %q", from, to, got, ok, want)
	}
}

func TestTheRefusalCarriesTheCauseThatNamesACaseDifference(t *testing.T) {
	got := caseOnlyRename(fakeRevision{"": {"policies"}, "policies": {"p.json"}}.entries, "policies/P.json")

	if got == nil || !strings.Contains(string(got.PolicyCause), "name_case_differs") {
		t.Errorf("refusal = %+v; want the name_case_differs cause", got)
	}
}
