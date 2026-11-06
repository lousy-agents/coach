package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

func TestRefuseUnsafeLazyFetchRefusesAnOldGitWhenARevisionItReadsLacksObjects(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		base       string
		words      []string
	}{
		{"two objects of HEAD", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing, secondMissing)}}}, "", []string{
			"git 2.43.0 cannot be told not to fetch missing objects", "HEAD has 2 objects that are not in it", "git 2.44 or later",
			"git rev-list --objects --missing=print --no-walk HEAD",
		}},
		{"one object of HEAD", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}}, "", []string{
			"HEAD has 1 object that is not in it",
		}},
		{"an object only the merge base lacks", fakeRepository{mergeBase: mergeBaseSHA, probes: map[string]probeAnswer{mergeBaseSHA: {output: missingIDs(firstMissing)}}}, "origin/main", []string{
			"the merge base of origin/main and HEAD has 1 object that is not in it", "--no-walk " + mergeBaseSHA,
		}},
		{"an object id of the longer hash", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: missingIDs(sha256Missing)}}}, "", []string{
			"HEAD has 1 object that is not in it",
		}},
		{"more objects than the output budget holds", fakeRepository{probes: map[string]probeAnswer{"HEAD": {err: &BoundError{Kind: BoundStdout, message: "git stdout exceeded the budget"}}}}, "", []string{
			"HEAD has many objects that are not in it",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRefusedFor(t, c.repository, c.base, c.words)
		})
	}
}

func assertRefusedFor(t *testing.T, repository fakeRepository, base string, words []string) {
	t.Helper()
	repository.version, repository.promisors = oldGit, promisorOrigin

	err := RefuseUnsafeLazyFetch(repository.run, "repo", base)

	var refusal *OperationalError
	if !errors.As(err, &refusal) {
		t.Fatalf("RefuseUnsafeLazyFetch = %v; want a refusal", err)
	}
	for _, word := range words {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
}

func TestRefuseUnsafeLazyFetchProbesAGitWhoseVersionCannotBeRead(t *testing.T) {
	repository := fakeRepository{version: "something else", promisors: promisorOrigin, probes: map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}}

	err := RefuseUnsafeLazyFetch(repository.run, "repo", "")

	if err == nil || !strings.Contains(err.Error(), "HEAD has 1 object that is not in it") {
		t.Errorf("RefuseUnsafeLazyFetch = %v; a git whose version cannot be read is treated as one that cannot be told, and probed", err)
	}
}

func TestTheRefusalOffersNoFetchCommandForARemoteNameThatCannotBePastedSafely(t *testing.T) {
	repository := fakeRepository{version: oldGit, promisors: "remote.origin\x1b[31m.promisor true\n", probes: map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}}

	err := RefuseUnsafeLazyFetch(repository.run, "repo", "")

	if err == nil || strings.Contains(err.Error(), "git fetch") || !strings.Contains(err.Error(), "git 2.44 or later") {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want the way out that is safe to paste, and no command built from a remote name that is not", err)
	}
}

// A base that cannot be shown on a terminal as it is must not reach one through the refusal.
func TestTheRefusalDoesNotPrintABaseThatHoldsAControlCharacter(t *testing.T) {
	repository := fakeRepository{version: oldGit, promisors: promisorOrigin, mergeBase: mergeBaseSHA, probes: map[string]probeAnswer{mergeBaseSHA: {output: missingIDs(firstMissing)}}}

	err := RefuseUnsafeLazyFetch(repository.run, "repo", "ba\x1b[31md")

	if err == nil || strings.Contains(err.Error(), "\x1b") || !strings.Contains(err.Error(), "the merge base of --base and HEAD") {
		t.Errorf("RefuseUnsafeLazyFetch = %q; want the merge base named without the base's own text", err)
	}
}
