package gitrepo

import (
	"errors"
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

var errUnparseableIndexLine = errors.New("git printed a line this version cannot parse")

// skipWorktreeTag is the status tag ls-files -t prints for a skip-worktree entry.
const skipWorktreeTag = "S"

// IndexEntries lists what the index holds for repoPath, relative to the
// repository root and matched literally. It returns none for a path the index
// does not know, and more than one for a path in a merge conflict. A line it
// cannot parse is an error: skipping it would read as an index that holds
// nothing, and guessing a field would read an entry as something it is not.
func IndexEntries(run Runner, dir, repoPath string) ([]IndexEntry, error) {
	output, err := run(dir, "ls-files", "--stage", "-t", "--", ":(top,literal)"+repoPath)
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: %s", repoPath, err), Cause: err}
	}
	listing := strings.TrimSpace(string(output))
	if listing == "" {
		return nil, nil
	}
	entries, err := parseIndexListing(listing)
	if err != nil {
		return nil, &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: %s", repoPath, err), Cause: err}
	}
	return entries, nil
}
