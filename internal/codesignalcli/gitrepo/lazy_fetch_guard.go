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

// noLazyFetchSupported reports whether the installed git reads GIT_NO_LAZY_FETCH, and the version it
// printed. A git that cannot be asked, or whose version cannot be read, is not taken to support it.
func noLazyFetchSupported(run Runner, dir string) (supported bool, version string) {
	output, err := run(dir, "version")
	if err != nil {
		return false, "unknown"
	}
	text := strings.TrimSpace(string(output))
	match := gitVersionPattern.FindStringSubmatch(text)
	if match == nil {
		return false, text
	}
	major, _ := strconv.Atoi(match[2])
	minor, _ := strconv.Atoi(match[3])
	return major > noLazyFetchMajor || (major == noLazyFetchMajor && minor >= noLazyFetchMinor), match[1]
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
// A git that can be told not to fetch never does, but a diff reads the merge base's version of every changed
// file, which the clone may lack: that is refused too, with the command that brings the objects in.
//
// On an older git, which ignores the instruction, a partial clone is read when every object of the
// revisions the scan reads is already here: base is empty for a baseline, which reads HEAD, and otherwise
// the scan reads HEAD and the merge base of base and HEAD. A revision that lacks objects is refused, and so
// is a question the probe cannot answer, because a read that might fetch is not made on a guess.
func RefuseMissingObjects(run Runner, dir, base string) error {
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
	supported, version := noLazyFetchSupported(run, dir)
	if supported {
		return refuseWhenTheMergeBaseLacksObjects(run, dir, base)
	}
	return refuseWhenReadRevisionsLackObjects(run, dir, base, version)
}

func refuseWhenReadRevisionsLackObjects(run Runner, dir, base, version string) error {
	revisions, err := revisionsRead(run, dir, base)
	if err != nil {
		return cannotTellWhatIsMissing(version, err)
	}
	for _, revision := range revisions {
		missing, err := missingObjectsOf(run, dir, revision.name)
		if err != nil {
			return cannotTellWhatIsMissing(version, err)
		}
		if missing.any() {
			return lacksObjects(run, dir, version, revision, missing)
		}
	}
	return nil
}

func lacksObjects(run Runner, dir, version string, revision readRevision, missing missingObjects) error {
	return &OperationalError{Message: fmt.Sprintf("this repository is a partial clone, git %s cannot be told not to fetch missing objects, and %s has %s not in it, so reading it could run its remote's transport and write objects; %s", version, revision.label, missing.phrase(), wayOut(FetchAdvice(run, dir, revision.name, "")))}
}

func cannotTellWhatIsMissing(version string, cause error) error {
	return &OperationalError{Message: fmt.Sprintf("could not tell whether this partial clone has the objects Coach reads (%s), and git %s cannot be told not to fetch missing objects, so a read could run its remote's transport and write objects; %s", TerminalSafe(cause.Error()), version, wayOut("")), Cause: cause}
}

// wayOut is the newer git that fixes the problem, and the fetch that brings the objects in when there is
// one that is safe to paste.
func wayOut(fetch string) string {
	upgrade := fmt.Sprintf("use git %d.%d or later", noLazyFetchMajor, noLazyFetchMinor)
	if fetch == "" {
		return upgrade
	}
	return upgrade + ", or " + fetch
}
