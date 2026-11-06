package projectconfig

import "strings"

// insideGitDirectory reports whether repoPath has a component that is git's own directory. Git tracks nothing
// there and refuses to add it, so such a path can be neither committed nor restored. A component is compared
// without regard to letter case, and without the trailing dots and spaces and the short name that Windows file
// systems fold to the same directory, because git refuses those spellings too.
func insideGitDirectory(repoPath string) bool {
	for _, component := range strings.Split(repoPath, "/") {
		if namesGitDirectory(component) {
			return true
		}
	}
	return false
}

func namesGitDirectory(component string) bool {
	name := strings.TrimRight(component, ". ")
	return strings.EqualFold(name, ".git") || strings.EqualFold(name, "git~1")
}
