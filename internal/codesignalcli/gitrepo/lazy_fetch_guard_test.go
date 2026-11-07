package gitrepo

import (
	"errors"
	"os/exec"
	"strconv"
	"testing"
)

// exitStatusOne is the error git leaves when `git config --get-regexp` matches nothing: a real exit 1 with
// nothing on stderr. exitStatus128 is the exit of a fatal error, with nothing on stderr either.
var exitStatusOne, exitStatus128 = exitingWith(1), exitingWith(128)

func exitingWith(code int) error {
	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		panic("sh did not fail with an exit status")
	}
	return err
}

func TestNoLazyFetchSupportedReadsTheVersionGitPrints(t *testing.T) {
	cases := []struct {
		name        string
		output      string
		err         error
		want        bool
		wantVersion string
	}{
		{"the first version that has it", "git version 2.44.0\n", nil, true, "2.44.0"},
		{"a patch release before it", "git version 2.43.5\n", nil, false, "2.43.5"},
		{"a vendor suffix", "git version 2.54.0 (Apple Git-157)\n", nil, true, "2.54.0"},
		{"a later major version", "git version 3.0.0\n", nil, true, "3.0.0"},
		{"a minor version compared as a number, not as text", "git version 2.9.1\n", nil, false, "2.9.1"},
		{"a three-digit minor version", "git version 2.100.0\n", nil, true, "2.100.0"},
		{"text that is not a version", "not git\n", nil, false, "not git"},
		{"words before the version", "xgit version 2.50.0\n", nil, false, "xgit version 2.50.0"},
		{"an older major version with a high minor", "git version 1.50.0\n", nil, false, "1.50.0"},
		{"a git that cannot be asked", "", errors.New("boom"), false, "unknown"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertVersionRead(t, c.output, c.err, c.want, c.wantVersion)
		})
	}
}

func assertVersionRead(t *testing.T, output string, err error, want bool, wantVersion string) {
	t.Helper()
	repository := &fakeRepository{version: output, versionErr: err}
	got, version := noLazyFetchSupported(repository.run, "repo")

	if got != want || version != wantVersion {
		t.Errorf("NoLazyFetchSupported = %v, %q; want %v, %q", got, version, want, wantVersion)
	}
}
