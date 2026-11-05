package gitrepo

import (
	"fmt"
	"strings"
)

// IndexEntry is one entry the index holds for a path. Stage is 0 for a path
// that is not in a merge conflict. SkipWorktree is set for a path a sparse
// checkout leaves out of the working tree.
type IndexEntry struct {
	TreeEntry
	Stage        string
	SkipWorktree bool
}

// skipWorktreeTag is the status tag ls-files -t prints for a skip-worktree entry.
const skipWorktreeTag = "S"

// IndexEntries lists what the index holds for repoPath, relative to the
// repository root and matched literally. It returns none for a path the index
// does not know, and more than one for a path in a merge conflict. A listing it
// cannot parse is an error: skipping a line would read as an index that holds
// nothing.
func IndexEntries(run func(dir string, args ...string) ([]byte, error), dir, repoPath string) ([]IndexEntry, error) {
	output, err := run(dir, "ls-files", "--stage", "-t", "--", ":(top,literal)"+repoPath)
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: %s", repoPath, err)}
	}
	listing := strings.TrimSpace(string(output))
	if listing == "" {
		return nil, nil
	}
	var entries []IndexEntry
	for _, line := range strings.Split(listing, "\n") {
		meta, _, found := strings.Cut(line, "\t")
		fields := strings.Fields(meta)
		if !found || len(fields) != 4 {
			return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: git printed a line this version cannot parse", repoPath)}
		}
		entries = append(entries, IndexEntry{TreeEntry: TreeEntry{Mode: fields[1], SHA: fields[2]}, Stage: fields[3], SkipWorktree: fields[0] == skipWorktreeTag})
	}
	return entries, nil
}
