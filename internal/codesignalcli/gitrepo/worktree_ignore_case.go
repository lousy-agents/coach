package gitrepo

import "strings"

// WorktreeIgnoreCase reports git's core.ignorecase, which `git init` sets to
// true on a filesystem that does not distinguish names by case.
func WorktreeIgnoreCase(dir string) (bool, error) {
	output, err := runWorktreeStatusGit(dir, "config", "--bool", "--default", "false", "--get", "core.ignorecase")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "true", nil
}
