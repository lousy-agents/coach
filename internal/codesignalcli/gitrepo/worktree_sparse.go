package gitrepo

import "strings"

// SparseState says which tracked paths a working-tree status listing cannot
// see: git omits skip-worktree and assume-unchanged paths from `git status`,
// whether the user set the bit or a sparse checkout did, so an edit to one is
// invisible to it.
type SparseState struct {
	SparseCheckout  bool
	SkipWorktree    bool
	AssumeUnchanged bool
}

// WorktreeSparseState reads core.sparseCheckout and the index's skip-worktree
// and assume-unchanged bits.
func WorktreeSparseState(dir string) (SparseState, error) {
	config, err := runWorktreeStatusGit(dir, "config", "--bool", "--default", "false", "--get", "core.sparseCheckout")
	if err != nil {
		return SparseState{}, err
	}
	tags, err := runWorktreeStatusGit(dir, "ls-files", "-v", "-z")
	if err != nil {
		return SparseState{}, err
	}
	return SparseState{
		SparseCheckout:  strings.TrimSpace(string(config)) == "true",
		SkipWorktree:    HasSkipWorktreeEntry(tags),
		AssumeUnchanged: HasAssumeUnchangedEntry(tags),
	}, nil
}

// HasSkipWorktreeEntry reports whether `git ls-files -v -z` output tags any
// entry S, the skip-worktree tag.
func HasSkipWorktreeEntry(output []byte) bool {
	for _, record := range SplitNULPaths(output) {
		if strings.HasPrefix(record, "S ") {
			return true
		}
	}
	return false
}

// HasAssumeUnchangedEntry reports whether `git ls-files -v -z` output tags any
// entry with a lowercase letter, which -v uses for assume-unchanged paths.
func HasAssumeUnchangedEntry(output []byte) bool {
	for _, record := range SplitNULPaths(output) {
		if len(record) > 2 && record[1] == ' ' && record[0] >= 'a' && record[0] <= 'z' {
			return true
		}
	}
	return false
}
