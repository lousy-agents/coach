package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// A git that can be told not to fetch never fetches, but a scan of a diff reads the merge base's version of
// every file that changed, and a partial clone may not have it. Left to the scan, the file is recorded as
// unreadable and the run exits 0 having analyzed none of the change: the check is made first.
func modernGitPartialClone() fakeRepository {
	return fakeRepository{version: currentGit, promisors: promisorOrigin, mergeBase: mergeBaseSHA}
}

func TestRefuseMissingObjectsRefusesADiffWhoseMergeBaseLacksObjectsOnAGitThatNeverFetches(t *testing.T) {
	cases := []struct {
		name  string
		probe probeAnswer
		words []string
	}{
		{"one object", probeAnswer{output: missingIDs(firstMissing)}, []string{"the merge base of origin/main and HEAD has 1 object that is not in it"}},
		{"two objects", probeAnswer{output: missingIDs(firstMissing, secondMissing)}, []string{"has 2 objects that are not in it"}},
		{"more than the output holds", probeAnswer{err: &BoundError{Kind: BoundStdout, message: "git stdout exceeded the budget"}}, []string{"has many objects that are not in it"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertMergeBaseRefused(t, c.probe, c.words)
		})
	}
}

func assertMergeBaseRefused(t *testing.T, probe probeAnswer, words []string) {
	t.Helper()
	repository := modernGitPartialClone()
	repository.probes = map[string]probeAnswer{mergeBaseSHA: probe}

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	var refusal *OperationalError
	if !errors.As(err, &refusal) {
		t.Fatalf("RefuseMissingObjects = %v; want a refusal", err)
	}
	want := append([]string{"Coach does not fetch objects; fetch the revision's files with for pass in 1 2 3 4; do ids=$({ git rev-list --objects --missing=print --no-walk " + mergeBaseSHA + "; git rev-list --objects --missing=print --no-walk HEAD; }", "and run again"}, words...)
	for _, word := range want {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
	if strings.Contains(err.Error(), "cannot be told") || strings.Contains(err.Error(), "2.44") {
		t.Errorf("refusal %q blames the git version: this git never fetches, and the objects are what is missing", err)
	}
}

func TestRefuseMissingObjectsAsksNothingOfAnOrdinaryClone(t *testing.T) {
	cases := map[string]struct {
		repository fakeRepository
		base       string
	}{
		"an ordinary clone has them all": {fakeRepository{version: currentGit, mergeBase: mergeBaseSHA}, "origin/main"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assertNothingProbedFor(t, c.repository, c.base)
		})
	}
}

func TestRefuseMissingObjectsAsksAboutNoMergeBaseForABaseline(t *testing.T) {
	repository := modernGitPartialClone()

	_ = RefuseMissingObjects(repository.run, "repo", "")

	for _, asked := range repository.mergeBaseAsked {
		t.Errorf("merge-base asked with %q for a baseline, which reads HEAD alone", asked)
	}
}

func assertNothingProbedFor(t *testing.T, repository fakeRepository, base string) {
	t.Helper()

	err := RefuseMissingObjects(repository.run, "repo", base)

	if err != nil || len(repository.probed) != 0 {
		t.Errorf("RefuseMissingObjects = %v after probing %q; want nothing asked", err, repository.probed)
	}
}
