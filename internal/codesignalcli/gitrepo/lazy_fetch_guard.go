package gitrepo

import (
	"fmt"
	"regexp"
	"slices"
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

// NoLazyFetchSupported reports whether the installed git reads GIT_NO_LAZY_FETCH, and the version it
// printed. A git that cannot be asked, or whose version cannot be read, is not taken to support it.
func NoLazyFetchSupported(run Runner, dir string) (supported bool, version string) {
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

// promisorConfiguration matches the two ways git marks a partial clone: remote.<name>.promisor, which
// current git writes, and extensions.partialclone, which older git wrote to name the promisor remote.
const promisorConfiguration = `^(extensions\.partialclone|remote\..*\.promisor)$`

// IsPartialClone reports whether the repository at dir was cloned with a filter, which is what makes git
// fetch a missing object on demand.
func IsPartialClone(run Runner, dir string) bool {
	output, err := run(dir, "config", "--get-regexp", promisorConfiguration)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(output), "\n") {
		key, value, _ := strings.Cut(strings.TrimSpace(line), " ")
		if strings.HasPrefix(key, "extensions.") && value != "" || strings.HasSuffix(key, ".promisor") && isTrue(value) {
			return true
		}
	}
	return false
}

func isTrue(value string) bool {
	return slices.Contains([]string{"true", "yes", "on", "1"}, strings.ToLower(value))
}

// RefuseUnsafeLazyFetch is nil unless dir is a partial clone and the installed git cannot be told not to
// fetch into it. An ordinary clone has nothing to fetch, so it is never refused.
func RefuseUnsafeLazyFetch(run Runner, dir string) error {
	if !IsPartialClone(run, dir) {
		return nil
	}
	supported, version := NoLazyFetchSupported(run, dir)
	if supported {
		return nil
	}
	return &OperationalError{Message: fmt.Sprintf("this repository is a partial clone, and git %s cannot be told not to fetch missing objects, so a read could run its remote's transport and write objects; use git %d.%d or later", version, noLazyFetchMajor, noLazyFetchMinor)}
}
