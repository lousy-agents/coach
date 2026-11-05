package gitrepo

import "strings"

// WorktreeGitlinks lists the tracked paths the index records as gitlinks, which
// is how it records a submodule.
func WorktreeGitlinks(dir string) ([]string, error) {
	output, err := runWorktreeStatusGit(dir, "ls-files", "-s", "-z")
	if err != nil {
		return nil, err
	}
	return ParseGitlinks(output), nil
}

// ParseGitlinks parses `git ls-files -s -z` output, whose records read
// "<mode> <object> <stage><TAB><path>", and returns the paths of mode 160000.
func ParseGitlinks(output []byte) []string {
	var gitlinks []string
	seen := map[string]bool{}
	for _, record := range SplitNULPaths(output) {
		header, path, found := strings.Cut(record, "\t")
		if !found || !strings.HasPrefix(header, "160000 ") || seen[path] {
			continue
		}
		seen[path] = true
		gitlinks = append(gitlinks, path)
	}
	return gitlinks
}
