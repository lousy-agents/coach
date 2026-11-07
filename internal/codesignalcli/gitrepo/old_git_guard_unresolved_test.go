package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

// A revision the scan will never read cannot fetch. One that does not resolve is a failure the scan ends on and
// words itself, the same on a git that can be told not to fetch and on one that cannot; a refusal that sends the
// reader to a newer git for it names a cause an upgrade does not touch.
func TestOnAnOldGitARevisionThatDoesNotResolveIsLeftToTheScan(t *testing.T) {
	headUnreadable := map[string]probeAnswer{"HEAD": {err: exitStatus128}}
	cases := []struct {
		name       string
		repository fakeRepository
		base       string
	}{
		{"a base that has no merge base", fakeRepository{mergeBaseErr: exitStatusOne}, "other"},
		{"a merge base that is not an object id", fakeRepository{mergeBase: "--upload-pack=x"}, "other"},
		{"a merge base that is an object id and then more", fakeRepository{mergeBase: mergeBaseSHA + " extra"}, "other"},
		{"a merge base that is an object id that is too short", fakeRepository{mergeBase: mergeBaseSHA[1:]}, "other"},
		{"a baseline of a repository with no commits", fakeRepository{unbornHead: true, probes: headUnreadable}, ""},
		{"a diff of a repository with no commits", fakeRepository{unbornHead: true, mergeBaseErr: exitStatus128, probes: headUnreadable}, "main"},
	}
	for _, c := range cases {
		assertLeftToTheScan(t, c.name, c.repository, c.base)
	}
}

func assertLeftToTheScan(t *testing.T, name string, repository fakeRepository, base string) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		repository.version = oldGit

		err := RefuseMissingObjects(repository.run, "repo", base)

		if err != nil {
			t.Errorf("RefuseMissingObjects = %v; want nil, so that the scan words the failure", err)
		}
	})
}

// Whether a revision resolves is a question too, and one git cannot be asked is not an answer of "no".
func TestOnAnOldGitAProbeThatFailsOnARevisionGitCannotBeAskedAboutIsRefused(t *testing.T) {
	repository := fakeRepository{version: oldGit, probes: map[string]probeAnswer{"HEAD": {err: exitStatus128}}}
	timedOut := &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}
	run := func(dir string, args ...string) ([]byte, error) {
		if args[0] == "rev-parse" {
			return nil, timedOut
		}
		return repository.run(dir, args...)
	}

	err := RefuseMissingObjects(run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(err.Error(), "could not tell whether") || !strings.Contains(err.Error(), "use git 2.44 or later") {
		t.Errorf("RefuseMissingObjects = %v; want the refusal for a probe that cannot be answered", err)
	}
}
