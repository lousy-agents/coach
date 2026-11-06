package gitrepo

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

const mergeBaseSHA = "feedfacefeedfacefeedfacefeedfacefeedface"

func TestRefuseUnsafeLazyFetchLeavesAReadAloneWhenNothingCanBeFetched(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
	}{
		{"a partial clone and a git that can be told not to fetch", fakeRepository{version: currentGit, promisors: promisorOrigin}},
		{"an ordinary clone and an old git", fakeRepository{version: oldGit}},
		{"a partial clone and an old git, every object of HEAD here", fakeRepository{version: oldGit, promisors: promisorOrigin}},
		{"a partial clone named by the extension older git wrote, every object here", fakeRepository{version: oldGit, extension: "origin\n"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNotRefused(t, c.repository)
		})
	}
}

func assertNotRefused(t *testing.T, repository fakeRepository) {
	t.Helper()

	if err := RefuseUnsafeLazyFetch(repository.run, "repo", ""); err != nil {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want a read that fetches nothing to go ahead", err)
	}
}

func TestRefuseUnsafeLazyFetchAsksNoQuestionOfGitWhenTheAnswerIsAlreadyKnown(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
	}{
		{"a git that can be told not to fetch", fakeRepository{version: currentGit, promisors: promisorOrigin}},
		{"a repository that is not a partial clone", fakeRepository{version: oldGit}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNothingProbed(t, c.repository)
		})
	}
}

func assertNothingProbed(t *testing.T, repository fakeRepository) {
	t.Helper()

	_ = RefuseUnsafeLazyFetch(repository.run, "repo", "origin/main")

	if len(repository.probed) != 0 {
		t.Errorf("probed %v; a read that cannot fetch has no objects to look for", repository.probed)
	}
}

// The probe is what makes the refusal depend on the objects a read needs: it lists the objects of each
// revision's own tree, and prints only the ones that are not local. --no-walk keeps it to that tree
// instead of every object in history, and --quiet keeps it to the missing ones.
func TestRefuseUnsafeLazyFetchProbesTheTreesTheScanReads(t *testing.T) {
	cases := []struct {
		name string
		base string
		want []string
	}{
		{"a baseline reads HEAD", "", []string{"rev-list --objects --missing=print --no-walk --quiet HEAD"}},
		{"a diff reads HEAD and the merge base", "origin/main", []string{
			"rev-list --objects --missing=print --no-walk --quiet HEAD",
			"rev-list --objects --missing=print --no-walk --quiet " + mergeBaseSHA,
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertProbed(t, c.base, c.want)
		})
	}
}

func assertProbed(t *testing.T, base string, want []string) {
	t.Helper()
	repository := fakeRepository{version: oldGit, promisors: promisorOrigin, mergeBase: mergeBaseSHA}

	if err := RefuseUnsafeLazyFetch(repository.run, "repo", base); err != nil {
		t.Fatalf("RefuseUnsafeLazyFetch = %v; want every object here to be read", err)
	}

	if !reflect.DeepEqual(repository.probed, want) {
		t.Errorf("probed %q; want %q", repository.probed, want)
	}
}

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

// A probe that fails says nothing about what is missing, and a read that might run the remote's transport
// is not made on a guess.
func TestRefuseUnsafeLazyFetchRefusesWhenItCannotTellWhatIsMissing(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		base       string
		cause      string
	}{
		{"the probe fails", fakeRepository{probes: map[string]probeAnswer{"HEAD": {err: errors.New("exit status 128: fatal: bad object HEAD")}}}, "", "bad object HEAD"},
		{"the probe runs out of time", fakeRepository{probes: map[string]probeAnswer{"HEAD": {err: &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}}}}, "", "timed out"},
		{"the probe prints what is not an object id", fakeRepository{probes: map[string]probeAnswer{"HEAD": {output: "0123456789012345678901234567890123456789 a.go\n"}}}, "", "a.go"},
		{"the merge base cannot be found", fakeRepository{mergeBaseErr: errors.New("exit status 1")}, "origin/main", "exit status 1"},
		{"the merge base is not an object id", fakeRepository{mergeBase: "--upload-pack=x"}, "origin/main", "--upload-pack=x"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRefusedForWantOfAnAnswer(t, c.repository, c.base, c.cause)
		})
	}
}

func assertRefusedForWantOfAnAnswer(t *testing.T, repository fakeRepository, base, cause string) {
	t.Helper()
	repository.version, repository.promisors = oldGit, promisorOrigin

	err := RefuseUnsafeLazyFetch(repository.run, "repo", base)

	var refusal *OperationalError
	if !errors.As(err, &refusal) || refusal.Cause == nil {
		t.Fatalf("RefuseUnsafeLazyFetch = %v; want a refusal that keeps the cause", err)
	}
	for _, word := range []string{"could not tell whether", "partial clone", "git 2.43.0", "git 2.44 or later", cause} {
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
