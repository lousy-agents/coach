package gitrepo

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// GIT_NO_LAZY_FETCH, which gitCommand sets, is read from git 2.44. An older git ignores it and fetches
// a missing object from a partial clone's promisor remote, which runs the remote's transport.
const (
	noLazyFetchMajor = 2
	noLazyFetchMinor = 44
)

var gitVersionPattern = regexp.MustCompile(`^git version ((\d+)\.(\d+)\S*)`)

// Coach passes --no-optional-locks to every git call (git 2.15) and asks rev-list --missing=print of every
// repository on a git that cannot be told not to fetch (git 2.16), so a scan needs 2.16 to start.
const (
	minimumGitMajor = 2
	minimumGitMinor = 16
)

// noLazyFetchSupported reports whether the installed git reads GIT_NO_LAZY_FETCH, and the version it
// printed. A git that cannot be asked, or whose version cannot be read, is not taken to support it.
func noLazyFetchSupported(run Runner, dir string) (supported bool, version string) {
	supported, version, _ = askGitsVersion(run, dir)
	return supported, version
}

// askGitsVersion is noLazyFetchSupported with the failure of the question kept: err is set only when git could not
// be asked at all. A version that is asked and cannot be read is a git that is not taken to support the
// instruction, and is not an error.
func askGitsVersion(run Runner, dir string) (supported bool, version string, err error) {
	output, err := run(dir, "version")
	if err != nil {
		return false, "unknown", err
	}
	text := strings.TrimSpace(string(output))
	match := gitVersionPattern.FindStringSubmatch(text)
	if match == nil {
		return false, text, nil
	}
	major, _ := strconv.Atoi(match[2])
	minor, _ := strconv.Atoi(match[3])
	return major > noLazyFetchMajor || (major == noLazyFetchMajor && minor >= noLazyFetchMinor), match[1], nil
}

// versionUnknownError is the refusal for a git that cannot say what version it is. The question is the first Coach
// asks and carries the options every call carries, so a git that fails it is one that does not know them, and every
// call after it would fail the same way, in words that point at the repository and not at the git: with the version
// unknown, a revision that cannot be read is taken for one that does not exist, and the scan says that the directory
// is not a worktree. A git that is not installed is said so as every other read says it.
func versionUnknownError(cause error) error {
	if errors.Is(cause, exec.ErrNotFound) {
		return gitNotFoundError()
	}
	return &OperationalError{Message: fmt.Sprintf("could not determine the version of git (%s); Coach needs git %d.%d or later", TerminalSafe(cause.Error()), minimumGitMajor, minimumGitMinor), Cause: cause}
}
