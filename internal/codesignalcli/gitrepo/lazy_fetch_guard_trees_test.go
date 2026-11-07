package gitrepo

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// A git that never fetches still cannot list a revision whose trees are not here: a clone made with
// --filter=tree:0 lacks the root tree itself, and the first read, `ls-tree`, fails with "not a tree object" and
// none of the guidance a missing blob gets. The trees of HEAD are asked about first, and blobs are left to the
// read that wants them, since a scan of a clone with no checkout needs only some of them.
const headTreesProbe = "rev-list --objects --missing=print --no-walk --filter=blob:none --quiet HEAD"

func TestRefuseMissingObjectsRefusesAScanWhoseHeadLacksTreesOnAGitThatNeverFetches(t *testing.T) {
	cases := []struct {
		name    string
		base    string
		listing string
	}{
		{"a baseline", "", "git rev-list --objects --missing=print --no-walk HEAD |"},
		{"a diff, whose merge base the clone lacks as well", "origin/main", "{ git rev-list --objects --missing=print --no-walk " + mergeBaseSHA + "; git rev-list --objects --missing=print --no-walk HEAD; }"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertHeadTreesRefused(t, c.base, c.listing)
		})
	}
}

func assertHeadTreesRefused(t *testing.T, base, listing string) {
	t.Helper()
	repository := modernGitPartialClone()
	repository.probes = map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}

	err := RefuseMissingObjects(repository.run, "repo", base)

	var refusal *OperationalError
	if !errors.As(err, &refusal) {
		t.Fatalf("RefuseMissingObjects = %v; want a refusal", err)
	}
	for _, word := range []string{
		"this repository is a partial clone, and HEAD has 1 object that is not in it, so its files cannot be listed",
		"Coach does not fetch objects; fetch the revision's files with for pass in",
		listing,
		"git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin",
		"and run again",
	} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
	if strings.Contains(err.Error(), "2.44") || strings.Contains(err.Error(), "cannot be told") {
		t.Errorf("refusal %q blames the git version: this git never fetches, and the trees are what is missing", err)
	}
}

func TestRefuseMissingObjectsAsksOnlyAboutTheTreesOfHeadBeforeTheMergeBaseOnAGitThatNeverFetches(t *testing.T) {
	cases := []struct {
		name string
		base string
		want []string
	}{
		{"a baseline", "", []string{headTreesProbe}},
		{"a diff", "origin/main", []string{headTreesProbe, "rev-list --objects --missing=print --no-walk --quiet " + mergeBaseSHA}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertOnlyHeadTreesProbed(t, c.base, c.want)
		})
	}
}

func assertOnlyHeadTreesProbed(t *testing.T, base string, want []string) {
	t.Helper()
	repository := modernGitPartialClone()

	err := RefuseMissingObjects(repository.run, "repo", base)

	if err != nil || !reflect.DeepEqual(repository.probed, want) {
		t.Errorf("RefuseMissingObjects = %v after probing %q; want %q: blobs are not asked about, the snapshot reports the ones it needs", err, repository.probed, want)
	}
}

// A HEAD that cannot be asked about is left to the scan, which says so in its own words: an empty repository has
// no HEAD, and refusing it as a partial clone would be the wrong sentence.
func TestRefuseMissingObjectsLeavesAHeadThatCannotBeAskedAboutToTheScan(t *testing.T) {
	repository := modernGitPartialClone()
	repository.probes = map[string]probeAnswer{"HEAD": {err: errors.New("fatal: ambiguous argument 'HEAD'")}}

	if err := RefuseMissingObjects(repository.run, "repo", ""); err != nil {
		t.Errorf("RefuseMissingObjects = %v; want the scan to say why HEAD cannot be read", err)
	}
}

// One paste has to bring in what the whole diff reads: the merge base the changed files are compared with, and
// HEAD, which the snapshot reads after it. Naming the merge base alone sent the user round again for HEAD.
func TestTheMergeBaseRefusalFetchesHeadToo(t *testing.T) {
	repository := modernGitPartialClone()
	repository.probes = map[string]probeAnswer{mergeBaseSHA: {output: missingIDs(firstMissing)}}

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	want := "{ git rev-list --objects --missing=print --no-walk " + mergeBaseSHA + "; git rev-list --objects --missing=print --no-walk HEAD; }"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("RefuseMissingObjects = %v; want a batch that lists %q", err, want)
	}
}

// A base that has no merge base is left to the scan to word, so the batch is for HEAD alone.
func TestTheHeadTreesRefusalFetchesHeadAloneWhenTheBaseHasNoMergeBase(t *testing.T) {
	repository := modernGitPartialClone()
	repository.mergeBaseErr = errors.New("exit status 1")
	repository.probes = map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}

	err := RefuseMissingObjects(repository.run, "repo", "no-such-ref")

	if err == nil || !strings.Contains(err.Error(), "ids=$(git rev-list --objects --missing=print --no-walk HEAD |") || strings.Contains(err.Error(), "{ git rev-list") {
		t.Errorf("RefuseMissingObjects = %v; want a batch for HEAD alone", err)
	}
}
