package gitrepo

import (
	"errors"
	"fmt"
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

var errIndexEntryForAnotherPath = errors.New("git listed an entry for a path other than the one asked about")

// skipWorktreeTag is the status tag ls-files -t prints for a skip-worktree entry.
const skipWorktreeTag = "S"

// IndexAt is what the index holds at a path and below it.
type IndexAt struct {
	// Entries are the entries for the path itself: none for a path the index does
	// not know, and more than one for a path in a merge conflict.
	Entries []IndexEntry
	// HoldsBelow is set when the index holds entries below the path, which makes the
	// path a directory there whatever else it holds.
	HoldsBelow bool
}

// IndexEntries lists what the index holds at repoPath, relative to the
// repository root and matched literally, and whether it holds anything below it.
// git matches a directory name to the files under it, so the listing carries each
// entry's own path and keeps the entries for repoPath apart from the ones below
// it: an entry for cfg/p.json says nothing about a path named cfg except that it
// is a directory. A line it cannot parse, or an entry for a path that is neither
// repoPath nor below it, is an error: skipping one would read as an index that
// holds nothing, and guessing would read an entry as something it is not. The
// listing is NUL-terminated, because git quotes a path with a newline, a tab or a
// quote in it unless it is asked not to, and a quoted name never equals repoPath.
// It names each path from the repository root, because git prints it relative to
// the directory it runs in otherwise, and a scan can start in a subdirectory.
func IndexEntries(run Runner, dir, repoPath string) (IndexAt, error) {
	output, err := run(dir, "ls-files", "--stage", "-t", "-z", "--full-name", "--", ":(top,literal)"+repoPath)
	if err != nil {
		return IndexAt{}, &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: %s", repoPath, err), Cause: err}
	}
	listed, err := parseIndexListing(output, repoPath)
	if err != nil {
		return IndexAt{}, &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: %s", repoPath, err), Cause: err}
	}
	return listed, nil
}
