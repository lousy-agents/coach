package gitrepo

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

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

// A base that looks like an option must be a revision to merge-base, not a flag: the -- comes first.
func TestTheMergeBaseIsAskedForWithDashDashBeforeABaseThatLooksLikeAnOption(t *testing.T) {
	repository := modernGitPartialClone()

	_ = RefuseMissingObjects(repository.run, "repo", "--help")

	want := [][]string{{"merge-base", "--", "--help", "HEAD"}}
	if !reflect.DeepEqual(repository.mergeBaseAsked, want) {
		t.Errorf("merge-base was asked %q; want %q", repository.mergeBaseAsked, want)
	}
}
