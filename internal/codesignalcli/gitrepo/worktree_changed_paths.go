package gitrepo

// ChangedPaths lists every path whose working-tree state may differ from HEAD:
// changed, added, deleted and untracked-but-not-ignored paths, plus the old path
// of a staged rename, which is gone from disk. A copy keeps its source, so only
// a rename yields it.
func ChangedPaths(entries []WorktreeEntry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
		if source, moved := entry.RenameSource(); moved {
			paths = append(paths, source)
		}
	}
	return paths
}
