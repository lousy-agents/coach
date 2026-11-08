package gitrepo

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// A git that is not installed is said so in the words every other read of it uses, whatever the scan asks
// first: the version, then for a diff the merge base, then the probe. It is not a git whose version is unknown,
// and not a question about partial clones.
func TestRefuseMissingObjectsSaysWhenGitIsNotInstalled(t *testing.T) {
	assertSaysGitIsNotInstalled(t, "a baseline", "")
	assertSaysGitIsNotInstalled(t, "a diff", "main")
}

func assertSaysGitIsNotInstalled(t *testing.T, scan, base string) {
	t.Helper()
	t.Run(scan, func(t *testing.T) {
		missing := &exec.Error{Name: "git", Err: exec.ErrNotFound}
		run := func(string, ...string) ([]byte, error) { return nil, missing }

		err := RefuseMissingObjects(run, "repo", base)

		var refusal *OperationalError
		if !errors.As(err, &refusal) || !strings.Contains(refusal.Message, "git executable not found") || strings.Contains(refusal.Message, "git unknown") {
			t.Errorf("RefuseMissingObjects = %v; want the message every read uses when git is not installed", err)
		}
	})
}
