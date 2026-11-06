package gitrepo

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestIsPartialCloneReadsEitherMarkerGitWrites(t *testing.T) {
	cases := []struct {
		name          string
		configuration string
		want          bool
	}{
		{"a promisor remote", promisorOrigin, true},
		{"a promisor remote spelled yes", "remote.origin.promisor yes\n", true},
		{"a promisor remote set to false", "remote.origin.promisor false\n", false},
		{"the extension older git wrote", "extensions.partialclone origin\n", true},
		{"no marker at all", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPartialCloneRead(t, c.configuration, c.want)
		})
	}
}

func assertPartialCloneRead(t *testing.T, configuration string, want bool) {
	t.Helper()
	if got, err := IsPartialClone(gitSaying("", configuration, nil), "repo"); err != nil || got != want {
		t.Errorf("IsPartialClone = %v, %v; want %v and no error", got, err, want)
	}
}

func TestIsPartialCloneSeesTheConfigurationGitWritesForAFilteredClone(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	clone, _ := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	ordinary := gitfixture.Init(t)

	if partial, err := IsPartialClone(RunBytes, clone); err != nil || !partial {
		t.Errorf("IsPartialClone(%s) = %v, %v; want true for a clone made with --filter", clone, partial, err)
	}
	if partial, err := IsPartialClone(RunBytes, ordinary); err != nil || partial {
		t.Errorf("IsPartialClone(%s) = %v, %v; want false for an ordinary repository", ordinary, partial, err)
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

func TestRefuseUnsafeLazyFetchRefusesARepositoryItCannotTellIsAPartialClone(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return nil, errors.New("exit status 128: fatal: bad config line 1 in file .git/config")
	}

	err := RefuseUnsafeLazyFetch(run, "repo")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "could not tell whether this repository is a partial clone") || refusal.Cause == nil {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want a refusal that says it could not tell whether the repository is a partial clone, and keeps git's error", err)
	}
}

// A git that is not installed is said so in the words every other read of it uses, not as a question about
// partial clones.
func TestRefuseUnsafeLazyFetchSaysWhenGitIsNotInstalled(t *testing.T) {
	missing := &exec.Error{Name: "git", Err: exec.ErrNotFound}
	run := func(string, ...string) ([]byte, error) { return nil, missing }

	err := RefuseUnsafeLazyFetch(run, "repo")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "git executable not found") {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want the message every read uses when git is not installed", err)
	}
}
