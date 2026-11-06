package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAPolicyMissingFromDiskIsRefusedWhenTheRevisionHoldsItsPathInAnotherCase(t *testing.T) {
	cases := []struct {
		name     string
		repoPath string
		revision revisionEntries
		want     []string
		avoid    []string
	}{
		{"a file name", "cfg/P.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries,
			[]string{`holds it as "cfg/p.json"; pass --project-config cfg/p.json`}, []string{"suggest"}},
		{"a directory name", "CFG/p.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries,
			[]string{`holds it as "cfg/p.json"; pass --project-config cfg/p.json`}, nil},
		{"both names", "CFG/P.JSON", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries,
			[]string{`holds it as "cfg/p.json"; pass --project-config cfg/p.json`}, nil},
		{"a name at the root", "P.json", fakeRevision{"": {"p.json"}}.entries,
			[]string{`holds it as "p.json"; pass --project-config p.json`}, nil},
		{"a revision that holds two spellings of the name", "cfg/P.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json", "P.JSON"}}.entries,
			[]string{`holds more than one spelling of it: "cfg/P.JSON", "cfg/p.json"; pass the one you mean`}, []string{"pass --project-config"}},
		{"a spelling that cannot be shown on a terminal", "cfg/P\x1b.json", fakeRevision{"": {"cfg"}, "cfg": {"p\x1b.json"}}.entries,
			[]string{`holds it as "cfg/p\x1b.json"`}, []string{"pass --project-config"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRespelledInRevision(t, c.repoPath, c.revision, c.want, c.avoid)
		})
	}
}

func assertRespelledInRevision(t *testing.T, repoPath string, revision revisionEntries, want, avoid []string) {
	t.Helper()
	got := respelledInRevision(revision, repoPath)

	if got == nil {
		t.Fatalf("respelledInRevision(%q) = nil; want a refusal", repoPath)
	}
	assertCaseMismatchAdvice(t, got, want, avoid)
}

func TestAPolicyMissingFromDiskStaysMissingWhenTheRevisionHoldsNoOtherSpellingOfIt(t *testing.T) {
	cases := []struct {
		name     string
		repoPath string
		revision revisionEntries
	}{
		{"the revision holds the path as typed", "cfg/p.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries},
		{"the revision holds no such name", "cfg/other.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries},
		{"the revision holds the typed spelling beside another case of it", "cfg/p.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json", "P.json"}}.entries},
		{"the revision holds no such directory", "docs/p.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries},
		{"the revision is empty", "cfg/p.json", fakeRevision{}.entries},
		{"the revision cannot be asked", "cfg/P.json", failingRevision},
		{"the name is respelled but the file below it is not held", "CFG/other.json", fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertStaysMissing(t, c.repoPath, c.revision)
		})
	}
}

func assertStaysMissing(t *testing.T, repoPath string, revision revisionEntries) {
	t.Helper()
	if got := respelledInRevision(revision, repoPath); got != nil {
		t.Errorf("respelledInRevision(%q) = %q; want nil, so the policy stays plainly missing", repoPath, got.Message)
	}
}

func TestOnlyAPolicyMissingForNoOtherReasonIsLookedUpInTheRevision(t *testing.T) {
	revision := fakeRevision{"": {"cfg"}, "cfg": {"p.json"}}.entries
	cases := []struct {
		name    string
		err     error
		entries revisionEntries
		advised bool
	}{
		{"a policy absent from disk", missingError("cfg/P.json"), revision, true},
		{"a policy whose parent is a file", blockedParentError("cfg/P.json", "cfg"), revision, false},
		{"a path that cannot be used", unusableOnDiskError("cfg/P.json", projectreadiness.CauseNotARegularFile, "is not a regular file"), revision, false},
		{"no way to ask the revision", missingError("cfg/P.json"), nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertAdvisedOnlyWhenMissing(t, c.err, c.entries, c.advised)
		})
	}
}

func assertAdvisedOnlyWhenMissing(t *testing.T, err error, entries revisionEntries, advised bool) {
	t.Helper()
	got := adviseOnRevision(err, "cfg/P.json", entries)

	if (got != err) != advised {
		t.Errorf("adviseOnRevision returned %v; want the error replaced = %v", got, advised)
	}
}
