package gitrepo

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

func TestIsPartialCloneReadsEitherMarkerGitWrites(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		want       bool
	}{
		{"a promisor remote", fakeRepository{promisors: promisorOrigin}, true},
		{"a promisor remote beside one that is not", fakeRepository{promisors: "remote.up.promisor false\nremote.origin.promisor true\n"}, true},
		{"a promisor remote set to false", fakeRepository{promisors: "remote.origin.promisor false\n"}, false},
		{"the extension older git wrote", fakeRepository{extension: "origin\n"}, true},
		{"no marker at all", fakeRepository{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPartialCloneRead(t, c.repository, c.want)
		})
	}
}

func assertPartialCloneRead(t *testing.T, repository fakeRepository, want bool) {
	t.Helper()
	if got, err := IsPartialClone(repository.run, "repo"); err != nil || got != want {
		t.Errorf("IsPartialClone = %v, %v; want %v and no error", got, err, want)
	}
}

// IsPartialClone reads a promisor setting the way git does, so it asks git to read it as a boolean. A
// key with no value at all is true to git, and so is any non-zero number; read as text, neither is.
func TestIsPartialCloneAsksGitToReadThePromisorSettingAsABoolean(t *testing.T) {
	var asked []string
	run := func(_ string, args ...string) ([]byte, error) {
		asked = append(asked, strings.Join(args, " "))
		return nil, exitStatusOne
	}

	_, _ = IsPartialClone(run, "repo")

	if !slices.ContainsFunc(asked, func(command string) bool {
		return strings.Contains(command, "--type=bool") && strings.Contains(command, "promisor")
	}) {
		t.Errorf("asked %q; want the promisor setting read with --type=bool", asked)
	}
}

// A question git cannot answer is not answered "no": git's exit 1 is its way of saying nothing matched, and
// anything else (a damaged config, a timeout, a bound) says Coach does not know.
func TestIsPartialCloneDoesNotTakeAFailureOfGitForNo(t *testing.T) {
	cases := map[string]error{
		"a damaged configuration":        errors.New("exit status 128: fatal: bad config line 1 in file .git/config"),
		"a fatal exit that said nothing": exitStatus128,
		"a read past the time bound":     &BoundError{Kind: BoundTimeout, message: "git execution timed out"},
		"a read past the stderr bound":   &BoundError{Kind: BoundStderr, message: "git stderr exceeded the budget"},
	}
	for name, failure := range cases {
		t.Run(name, func(t *testing.T) { requireUnanswered(t, failure) })
	}
}

func requireUnanswered(t *testing.T, failure error) {
	t.Helper()
	run := func(string, ...string) ([]byte, error) { return nil, failure }
	if partial, err := IsPartialClone(run, "repo"); err == nil || partial {
		t.Errorf("IsPartialClone = %v, %v; want an error and no answer", partial, err)
	}
}

// A git that cannot be asked anything, its version included, is taken for one that cannot be told not to fetch,
// and what it holds cannot be told either, so the read is refused with git's own error kept.
func TestRefuseMissingObjectsRefusesARepositoryGitCannotBeAskedAbout(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 128: fatal: bad config line 1 in file .git/config")
	}

	err := RefuseMissingObjects(run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "could not tell whether this repository has the objects Coach reads") || refusal.Cause == nil {
		t.Errorf("RefuseMissingObjects = %v; want a refusal that says it could not tell what the repository holds, and keeps git's error", err)
	}
}

// A repository whose configuration git cannot read, on a git that can be told not to fetch, is still refused for
// it: nothing says it is an ordinary clone.
func TestRefuseMissingObjectsRefusesAConfigurationGitCannotReadOnAGitThatNeverFetches(t *testing.T) {
	repository := fakeRepository{version: currentGit, configErr: errors.New("exit status 128: fatal: bad config line 1 in file .git/config")}

	err := RefuseMissingObjects(repository.run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "could not tell whether this repository is a partial clone") || refusal.Cause == nil {
		t.Errorf("RefuseMissingObjects = %v; want a refusal that says it could not tell whether the repository is a partial clone, and keeps git's error", err)
	}
}

// A git that is not installed is said so in the words every other read of it uses, not as a question about
// partial clones.
func TestRefuseMissingObjectsSaysWhenGitIsNotInstalled(t *testing.T) {
	missing := &exec.Error{Name: "git", Err: exec.ErrNotFound}
	run := func(string, ...string) ([]byte, error) { return nil, missing }

	err := RefuseMissingObjects(run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "git executable not found") {
		t.Errorf("RefuseMissingObjects = %v; want the message every read uses when git is not installed", err)
	}
}
