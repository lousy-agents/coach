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

// ConfigHasNoValue is whether err is git config's exit 1, which is how it says that nothing matched: an
// unset key, or no key that matches. Every other failure (a damaged configuration, a time or output bound)
// leaves the question unanswered.
func ConfigHasNoValue(err error) bool {
	var exit *exec.ExitError
	return errors.As(err, &exit) && exit.ExitCode() == 1
}

// RefuseMissingObjects is nil unless the scan would read an object a partial clone lacks. An ordinary clone
// has every object; a repository that cannot be told apart from an ordinary one is refused, because reading it
// could run its remote's transport.
//
// A git that can be told not to fetch never does, so whether the repository is a partial clone only decides
// whether to look: a clone made without trees lacks the trees of HEAD, which nothing can be listed without, and
// a diff reads the merge base's version of every changed file, which the clone may lack. Both are refused, with
// the command that brings the objects in.
//
// On an older git, which ignores the instruction, the configuration decides nothing. What git takes for a
// promisor remote is more than the settings Coach reads (a remote with a partial clone filter is one, with
// promisor unset or false), and a read of a missing object from it fetches. So the revisions the scan reads are
// always asked about, with a probe that never fetches, and a revision that lacks objects is refused, whatever
// the configuration says; it only words the refusal. base is empty for a baseline, which reads HEAD, and
// otherwise the scan reads HEAD and the merge base of base and HEAD. A question the probe cannot answer is a
// refusal too, because a read that might fetch is not made on a guess.
func RefuseMissingObjects(run Runner, dir, base string) error {
	supported, version, err := askGitsVersion(run, dir)
	if err != nil {
		return versionUnknownError(err)
	}
	if !supported {
		return refuseWhenReadRevisionsLackObjects(run, dir, base, version)
	}
	partial, err := IsPartialClone(run, dir)
	if errors.Is(err, exec.ErrNotFound) {
		return gitNotFoundError()
	}
	if err != nil {
		return &OperationalError{Message: fmt.Sprintf("could not tell whether this repository is a partial clone (%s), so it is not read: a partial clone's read could run its remote's transport and write objects", TerminalSafe(err.Error())), Cause: err}
	}
	if !partial {
		return nil
	}
	if err := refuseWhenHeadLacksTrees(run, dir, base); err != nil {
		return err
	}
	return refuseWhenTheMergeBaseLacksObjects(run, dir, base)
}
