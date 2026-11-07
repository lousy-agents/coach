package gitrepo

import (
	"errors"
	"reflect"
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
	want := append([]string{"Coach does not fetch objects; fetch the revision's files with git rev-list --objects --missing=print --no-walk " + mergeBaseSHA, "and run again"}, words...)
	for _, word := range want {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
	if strings.Contains(err.Error(), "cannot be told") || strings.Contains(err.Error(), "2.44") {
		t.Errorf("refusal %q blames the git version: this git never fetches, and the objects are what is missing", err)
	}
}

func TestRefuseMissingObjectsReadsTheMergeBaseAloneOnAGitThatNeverFetches(t *testing.T) {
	repository := modernGitPartialClone()

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	want := []string{"rev-list --objects --missing=print --no-walk --quiet " + mergeBaseSHA}
	if err != nil || !reflect.DeepEqual(repository.probed, want) {
		t.Errorf("RefuseMissingObjects = %v after probing %q; want a diff to go ahead having asked about the merge base alone: HEAD's objects are read, and reported, by the snapshot", err, repository.probed)
	}
}

func TestRefuseMissingObjectsAsksAboutNoMergeBaseForABaselineOrAnOrdinaryClone(t *testing.T) {
	cases := map[string]struct {
		repository fakeRepository
		base       string
	}{
		"a baseline reads HEAD alone":    {modernGitPartialClone(), ""},
		"an ordinary clone has them all": {fakeRepository{version: currentGit, mergeBase: mergeBaseSHA}, "origin/main"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assertNothingProbedFor(t, c.repository, c.base)
		})
	}
}

func assertNothingProbedFor(t *testing.T, repository fakeRepository, base string) {
	t.Helper()

	err := RefuseMissingObjects(repository.run, "repo", base)

	if err != nil || len(repository.probed) != 0 {
		t.Errorf("RefuseMissingObjects = %v after probing %q; want nothing asked", err, repository.probed)
	}
}

// The scan words an unresolvable base itself, so a merge base that cannot be found is left to it. A probe
// that fails once the merge base is known is not: the scan would go on, and record every changed file as
// unreadable.
func TestRefuseMissingObjectsLeavesABaseThatHasNoMergeBaseToTheScan(t *testing.T) {
	repository := modernGitPartialClone()
	repository.mergeBaseErr = errors.New("exit status 1")

	err := RefuseMissingObjects(repository.run, "repo", "no-such-ref")

	if err != nil || len(repository.probed) != 0 {
		t.Errorf("RefuseMissingObjects = %v after probing %q; want the scan to say the base cannot be resolved", err, repository.probed)
	}
}

func TestRefuseMissingObjectsRefusesWhenItCannotTellWhatTheMergeBaseLacks(t *testing.T) {
	repository := modernGitPartialClone()
	repository.probes = map[string]probeAnswer{mergeBaseSHA: {err: errors.New("exit status 128: fatal: bad object")}}

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || refusal.Cause == nil {
		t.Fatalf("RefuseMissingObjects = %v; want a refusal that keeps git's own error", err)
	}
	for _, word := range []string{"could not tell whether", "the merge base of origin/main and HEAD", "bad object"} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
}

// The fetch command is built from the remote's name, so a name that cannot be pasted safely leaves the refusal
// without one, and without the separator that would have introduced it.
func TestTheMergeBaseRefusalOffersNoFetchCommandForARemoteNameThatCannotBePastedSafely(t *testing.T) {
	repository := modernGitPartialClone()
	repository.promisors = "remote.origin\x1b[31m.promisor true\n"
	repository.probes = map[string]probeAnswer{mergeBaseSHA: {output: missingIDs(firstMissing)}}

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	if err == nil || !strings.HasSuffix(err.Error(), "Coach does not fetch objects") || strings.Contains(err.Error(), "git fetch") {
		t.Errorf("RefuseMissingObjects = %v; want the refusal with no command and no trailing separator", err)
	}
}
