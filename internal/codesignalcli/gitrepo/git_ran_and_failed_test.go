package gitrepo

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A git that ran to the end and failed is told apart from one that could not be asked, by what happened and not
// by what it said: a failure with a message on stderr (outside a repository, a bad object) is the same kind of
// failure as a silent one, and neither is an answer from a git that did not finish.
func TestAGitThatRanAndFailedIsToldApartFromOneThatCouldNotBeAsked(t *testing.T) {
	notARepository := t.TempDir()
	repository := gitfixture.Init(t)
	_, silent := RunBytes(repository, "rev-parse", "--verify", "--quiet", "HEAD")
	_, withMessage := RunBytes(notARepository, "rev-parse", "--verify", "--quiet", "HEAD")
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"a silent failure", silent, true},
		{"a failure with a message on stderr", withMessage, true},
		{"a git that ran out of time", &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}, false},
		{"a git that is not installed", &exec.Error{Name: "git", Err: exec.ErrNotFound}, false},
		{"no error", nil, false},
	}
	for _, c := range cases {
		assertRanAndFailed(t, c.name, c.err, c.want)
	}
}

func assertRanAndFailed(t *testing.T, name string, err error, want bool) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		if got := gitRanAndFailed(err); got != want {
			t.Errorf("gitRanAndFailed(%v) = %v; want %v", err, got, want)
		}
	})
}

// A failure with a message is still not the silent exit 1 that git config and cat-file -e use to answer "no":
// the message says it is an error.
func TestAFailureWithAMessageIsNotTheSilentExitThatMeansNo(t *testing.T) {
	_, err := RunBytes(t.TempDir(), "rev-parse", "--verify", "--quiet", "HEAD")

	var exit *exec.ExitError
	if errors.As(err, &exit) || ConfigHasNoValue(err) {
		t.Errorf("%v is read as git's silent exit; a failure that printed a message is not one", err)
	}
}
