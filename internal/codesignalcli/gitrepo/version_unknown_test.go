package gitrepo

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// The question of the version carries the options every call carries, so a git that fails it does not know them.
// It is refused for what it is, with what git said and the git Coach needs, and not passed on to the questions
// after it, which would fail the same way and be read as a revision that does not exist.
func TestAGitThatFailsTheQuestionOfItsVersionIsRefusedAsAGitThatCannotBeRead(t *testing.T) {
	said := errors.New("exit status 129: unknown option: --no-optional-locks\nusage: git [--version] [--help]")
	repository := fakeRepository{versionErr: said}

	err := RefuseMissingObjects(repository.run, "repo", "")

	var refusal *OperationalError
	if !errors.As(err, &refusal) || !errors.Is(refusal.Cause, said) {
		t.Fatalf("RefuseMissingObjects = %v; want a refusal that keeps what git said", err)
	}
	for _, word := range []string{"could not determine the version of git", "unknown option: --no-optional-locks", "git 2.16 or later"} {
		if !strings.Contains(err.Error(), word) {
			t.Errorf("refusal %q does not say %q", err, word)
		}
	}
	if len(repository.probed) != 0 || len(repository.mergeBaseAsked) != 0 {
		t.Errorf("after the version failed, git was asked %q and merge-base %v; want nothing asked", repository.probed, repository.mergeBaseAsked)
	}
}

func TestAGitThatCannotBeStartedForTheVersionIsSaidNotToBeInstalled(t *testing.T) {
	repository := fakeRepository{versionErr: &exec.Error{Name: "git", Err: exec.ErrNotFound}}

	err := RefuseMissingObjects(repository.run, "repo", "")

	if err == nil || !strings.Contains(err.Error(), "git executable not found") || strings.Contains(err.Error(), "version of git") {
		t.Errorf("RefuseMissingObjects = %v; want the message every read uses when git is not installed", err)
	}
}

func TestAGitThatRunsOutOfTimeForTheVersionIsRefusedAsOneThatCannotBeRead(t *testing.T) {
	repository := fakeRepository{versionErr: &BoundError{Kind: BoundTimeout, message: "git execution timed out after 30s"}}

	err := RefuseMissingObjects(repository.run, "repo", "")

	if err == nil || !strings.Contains(err.Error(), "could not determine the version of git (git execution timed out after 30s)") {
		t.Errorf("RefuseMissingObjects = %v; want the timeout named", err)
	}
}

// What git printed is shown to a terminal, so a control character in it is shown as text.
func TestWhatAGitSaidAboutItsVersionIsMadeSafeForATerminal(t *testing.T) {
	repository := fakeRepository{versionErr: errors.New("exit status 129: \x1b]0;PWNED\x07")}

	err := RefuseMissingObjects(repository.run, "repo", "")

	if err == nil || strings.ContainsAny(err.Error(), "\x1b\x07") {
		t.Errorf("RefuseMissingObjects = %q; want no control character git printed", err)
	}
}

// A version that is asked and printed in a form Coach does not know is not a git that failed: it is taken not to
// support the instruction, and the guard goes on as for an older git.
func TestAVersionThatIsPrintedAndNotUnderstoodIsNotAFailureToAskIt(t *testing.T) {
	repository := fakeRepository{version: "git version garbage", probes: map[string]probeAnswer{"HEAD": {output: missingIDs(firstMissing)}}}

	err := RefuseMissingObjects(repository.run, "repo", "")

	if err == nil || strings.Contains(err.Error(), "could not determine the version of git") || !strings.Contains(err.Error(), "cannot be told not to fetch missing objects") {
		t.Errorf("RefuseMissingObjects = %v; want the guard of an older git, not a failure to ask", err)
	}
}
