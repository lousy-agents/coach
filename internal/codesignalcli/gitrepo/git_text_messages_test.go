package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Every place git's words are put into a message shows a control character in them as an escape.

func TestGitWaitErrorQuotesWhatGitSaid(t *testing.T) {
	err := gitWaitError(errors.New("exit status 128"), []byte("fatal: bad "+rightToLeftOverride+"name "+eightBitCSI+"2J\n"))

	if strings.Contains(err.Error(), rightToLeftOverride) || strings.Contains(err.Error(), eightBitCSI) {
		t.Errorf("gitWaitError = %q holds a control character git printed", err)
	}
	if !strings.Contains(err.Error(), "\\u202e") || !strings.HasPrefix(err.Error(), "exit status 128: ") {
		t.Errorf("gitWaitError = %q, want the exit status and the escape of the override", err)
	}
}

func TestGitWaitErrorKeepsOrdinaryGitWordsReadable(t *testing.T) {
	err := gitWaitError(errors.New("exit status 128"), []byte("fatal: not a git repository\n"))

	if err.Error() != "exit status 128: fatal: not a git repository" {
		t.Errorf("gitWaitError = %q, want git's words as they are", err)
	}
}

func TestRefuseMissingObjectsQuotesWhatGitSaid(t *testing.T) {
	run := func(string, ...string) ([]byte, error) {
		return nil, errors.New("fatal: bad " + rightToLeftOverride + "name " + eightBitCSI + "2J")
	}

	err := RefuseMissingObjects(run, "repo", "")

	if err == nil || strings.Contains(err.Error(), rightToLeftOverride) || strings.Contains(err.Error(), eightBitCSI) {
		t.Errorf("RefuseMissingObjects = %v, want a refusal that shows git's control characters as escapes", err)
	}
}

func TestRefuseMissingObjectsQuotesWhatGitSaidWhenItCannotListWhatIsMissing(t *testing.T) {
	run := func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "config":
			return []byte("remote.origin.promisor true\n"), nil
		case "version":
			return []byte("git version 2.43.0"), nil
		}
		return nil, errors.New("fatal: bad " + rightToLeftOverride + "name " + eightBitCSI + "2J")
	}

	err := RefuseMissingObjects(run, "repo", "")

	if err == nil || strings.Contains(err.Error(), rightToLeftOverride) || strings.Contains(err.Error(), eightBitCSI) {
		t.Errorf("RefuseMissingObjects = %v, want a refusal that shows git's control characters as escapes", err)
	}
}

// The merge base is asked about on every git for a diff, so its refusals show git's words the same way.
func TestRefuseMissingObjectsQuotesWhatGitSaidWhenItCannotListWhatTheMergeBaseLacks(t *testing.T) {
	repository := modernGitPartialClone()
	repository.probes = map[string]probeAnswer{mergeBaseSHA: {err: errors.New("fatal: bad " + rightToLeftOverride + "name " + eightBitCSI + "2J")}}

	err := RefuseMissingObjects(repository.run, "repo", "origin/main")

	if err == nil || strings.Contains(err.Error(), rightToLeftOverride) || strings.Contains(err.Error(), eightBitCSI) || !strings.Contains(err.Error(), "\\u202e") {
		t.Errorf("RefuseMissingObjects = %v, want a refusal that shows git's control characters as escapes", err)
	}
}

func TestReadObjectsQuotesWhatCatFileSaid(t *testing.T) {
	// answers one object, then fails with a message that holds a right-to-left override (bytes e2 80 ae)
	script := "printf '0000 blob 1\\nx\\n'; printf 'fatal: bad \\342\\200\\256name\\n' >&2; exit 128"
	withStreamChild(t, func(ctx context.Context) *exec.Cmd { return exec.CommandContext(ctx, "sh", "-c", script) })

	_, readErr := readObjects("", []string{"0000"}, snapshotBudget{wall: time.Minute, object: 1 << 20}, func(string, []byte) error { return nil })

	if readErr == nil || strings.Contains(readErr.Error(), rightToLeftOverride) || !strings.Contains(readErr.Error(), "\\u202e") {
		t.Errorf("readObjects = %v, want git's override shown as the escape \\u202e", readErr)
	}
}
