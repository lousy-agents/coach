package gitrepo

import "strings"

// SparseState says which tracked paths a working-tree status listing cannot
// see: git omits skip-worktree and assume-unchanged paths from `git status`,
// whether the user set the bit or a sparse checkout did, so an edit to one is
// invisible to it. Tracked is every path in the index, so a caller can tell a
// directory git hides entirely from one it hides in part.
type SparseState struct {
	SparseCheckout  bool
	Tracked         []string
	SkipWorktree    []string
	AssumeUnchanged []string
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
	state := ParseIndexTags(tags)
	state.SparseCheckout = strings.TrimSpace(string(config)) == "true"
	return state, nil
}

// ParseIndexTags sorts the paths of `git ls-files -v -z` output by tag. S is the
// skip-worktree tag; -v spells assume-unchanged as a lowercase letter, so a path
// with both bits is tagged s and belongs to both lists.
func ParseIndexTags(output []byte) SparseState {
	var state SparseState
	for _, record := range SplitNULPaths(output) {
		if len(record) <= 2 || record[1] != ' ' {
			continue
		}
		path := record[2:]
		state.Tracked = append(state.Tracked, path)
		switch tag := record[0]; {
		case tag == 'S':
			state.SkipWorktree = append(state.SkipWorktree, path)
		case tag == 's':
			state.SkipWorktree = append(state.SkipWorktree, path)
			state.AssumeUnchanged = append(state.AssumeUnchanged, path)
		case tag >= 'a' && tag <= 'z':
			state.AssumeUnchanged = append(state.AssumeUnchanged, path)
		}
	}
	return state
}
