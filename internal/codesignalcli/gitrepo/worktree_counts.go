package gitrepo

// WorktreeCounts is how many distinct paths the working tree changes, by
// category, over every language.
type WorktreeCounts struct {
	Modified  int
	Added     int
	Deleted   int
	Untracked int
}

// CountEntries counts the distinct paths in a working-tree status. A path has
// one category, the first that applies: untracked, deleted (either column),
// added (index column A), modified. An unmerged path counts as modified, because
// HEAD and the disk both hold a version of it; only a path both sides deleted is
// gone, and counts as deleted. Git lists a path twice when it is removed from the
// index but kept on disk, as deleted and as untracked; it counts once, as
// untracked. A staged rename is one deleted path, the old one, and one added
// path, the new one; a copy adds its new path only. An added, renamed or copied
// path that was then deleted from disk is in neither HEAD nor the working tree,
// so it counts nowhere; such a rename still deletes its old path. Ignored paths
// do not appear in a status, and neither do the paths git hides (sparse,
// skip-worktree, assume-unchanged, submodule and nested-repository paths).
func CountEntries(entries []WorktreeEntry) WorktreeCounts {
	category := map[string]worktreeCategory{}
	note := func(path string, found worktreeCategory) {
		if current, seen := category[path]; !seen || found < current {
			category[path] = found
		}
	}
	for _, entry := range entries {
		if !NewPathDeletedFromDisk(entry.Code) {
			note(entry.Path, categoryOf(entry))
		}
		if source, moved := entry.RenameSource(); moved {
			note(source, categoryDeleted)
		}
	}
	var counts WorktreeCounts
	for _, found := range category {
		counts.add(found)
	}
	return counts
}

// worktreeCategory is ordered by precedence: a path counts under the lowest
// category that applies to it.
type worktreeCategory int

const (
	categoryUntracked worktreeCategory = iota
	categoryDeleted
	categoryAdded
	categoryModified
)

func categoryOf(entry WorktreeEntry) worktreeCategory {
	switch {
	case entry.IsUntracked():
		return categoryUntracked
	case entry.Code == "DD":
		return categoryDeleted
	case entry.IsUnmerged():
		return categoryModified
	case entry.IsDeletedFromDisk():
		return categoryDeleted
	case entry.IsNew():
		return categoryAdded
	}
	return categoryModified
}

func (c *WorktreeCounts) add(found worktreeCategory) {
	switch found {
	case categoryUntracked:
		c.Untracked++
	case categoryDeleted:
		c.Deleted++
	case categoryAdded:
		c.Added++
	default:
		c.Modified++
	}
}
