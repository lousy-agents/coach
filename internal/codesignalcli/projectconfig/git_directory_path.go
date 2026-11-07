package projectconfig

import (
	"strings"

	"github.com/lousy-agents/coach/internal/gitpath"
)

// insideGitDirectory reports whether repoPath has a component that is git's own directory. Git tracks nothing
// there and refuses to add it, so such a path can be neither committed nor restored. The spellings are those
// git refuses (gitpath.NamesGitDirectory): letter case, the trailing dots and spaces and the short name that
// Windows file systems fold to the same directory, and the code points that HFS+ ignores.
func insideGitDirectory(repoPath string) bool {
	for _, component := range strings.Split(repoPath, "/") {
		if gitpath.NamesGitDirectory(component) {
			return true
		}
	}
	return false
}
