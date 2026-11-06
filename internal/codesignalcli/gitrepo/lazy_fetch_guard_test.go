package gitrepo

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
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

// gitSaying is a runner that answers `git version` and the promisor configuration with the given
// output, or with git's exit 1 when it is empty.
func gitSaying(version, configuration string, versionErr error) Runner {
	return func(_ string, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "version" {
			return []byte(version), versionErr
		}
		if configuration == "" {
			return nil, exitStatusOne
		}
		return []byte(configuration), nil
	}
}

const promisorOrigin = "remote.origin.promisor true\nremote.origin.partialclonefilter blob:none\n"

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
	got, version := noLazyFetchSupported(gitSaying(output, "", err), "repo")

	if got != want || version != wantVersion {
		t.Errorf("NoLazyFetchSupported = %v, %q; want %v, %q", got, version, want, wantVersion)
	}
}

func TestRefuseUnsafeLazyFetchRefusesOnlyAPartialCloneWhoseGitCannotBeToldNotToFetch(t *testing.T) {
	cases := []struct {
		name         string
		version      string
		partialClone string
		wantRefused  bool
	}{
		{"a partial clone and an old git", "git version 2.43.0", promisorOrigin, true},
		{"a partial clone and a git that can be told", "git version 2.44.0", promisorOrigin, false},
		{"an ordinary clone and an old git", "git version 2.43.0", "", false},
		{"a partial clone and a version nobody can read", "something else", promisorOrigin, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRefusedOrNot(t, c.version, c.partialClone, c.wantRefused)
		})
	}
}

func assertRefusedOrNot(t *testing.T, version, partialClone string, wantRefused bool) {
	t.Helper()
	err := RefuseUnsafeLazyFetch(gitSaying(version, partialClone, nil), "repo")

	if (err != nil) != wantRefused {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want refused = %v", err, wantRefused)
	}
}

func TestTheRefusalNamesTheVersionAndTheOneThatIsNeeded(t *testing.T) {
	err := RefuseUnsafeLazyFetch(gitSaying("git version 2.43.0", promisorOrigin, nil), "repo")

	if err == nil || !strings.Contains(err.Error(), "git 2.43.0 cannot be told") || !strings.Contains(err.Error(), "git 2.44 or later") || !strings.Contains(err.Error(), "partial clone") {
		t.Errorf("RefuseUnsafeLazyFetch = %v; want the version, the version needed and the reason", err)
	}
}
