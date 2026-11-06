package gitrepo

import (
	"errors"
	"fmt"
	"slices"
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

var errIndexEntryForAnotherPath = errors.New("git listed an entry for a path other than the one asked about")

// skipWorktreeTag is the status tag ls-files -t prints for a skip-worktree entry.
const skipWorktreeTag = "S"

// IndexAt is what the index holds at a path and below it.
type IndexAt struct {
	// Entries are the entries for the path itself: none for a path the index does
	// not know, and more than one for a path in a merge conflict.
	Entries []IndexEntry
	// HoldsBelow is set when the path has no entry of its own and the index holds
	// entries below it, which makes it a directory there.
	HoldsBelow bool
}

// IndexEntries says what the index holds at repoPath, relative to the repository
// root and matched literally: the entries for the path itself and, when there are
// none, whether it holds anything below it, which makes the path a directory.
// git matches a directory name to the files under it, so the entries are asked for
// with the entries below the path excluded, and each record carries its own path:
// an entry for cfg/p.json says nothing about a path named cfg except that it is a
// directory. A line it cannot parse, or an entry for any other path, is an error:
// skipping one would read as an index that holds nothing, and guessing would read an
// entry as something it is not. The listing is NUL-terminated, because git quotes a
// path with a newline, a tab or a quote in it unless it is asked not to, and a quoted
// name never equals repoPath. It names each path from the repository root, because git
// prints it relative to the directory it runs in otherwise, and a scan can start in a
// subdirectory.
func IndexEntries(run Runner, dir, repoPath string) (IndexAt, error) {
	entries, err := entriesAtPath(run, dir, repoPath)
	if err != nil || len(entries) > 0 {
		return IndexAt{Entries: entries}, err
	}
	below, err := holdsEntriesBelow(run, dir, repoPath)
	return IndexAt{HoldsBelow: below}, err
}

func entriesAtPath(run Runner, dir, repoPath string) ([]IndexEntry, error) {
	output, err := run(dir, "ls-files", "--stage", "-t", "-z", "--full-name", "--", ":(top,literal)"+repoPath, ":(top,exclude,literal)"+repoPath+"/")
	if err != nil {
		return nil, indexReadError(repoPath, err)
	}
	entries, err := parseIndexListing(output, repoPath)
	if err != nil {
		return nil, indexReadError(repoPath, err)
	}
	return entries, nil
}

// holdsEntriesBelow is whether the index holds anything below repoPath. It takes
// one entry to say yes, so a listing that overruns the output budget answers it
// too: a budget sized for a policy document must not decide whether a large
// directory is a directory. Any other failure, or an entry for another path, is an
// error.
func holdsEntriesBelow(run Runner, dir, repoPath string) (bool, error) {
	output, err := run(dir, "ls-files", "-z", "--full-name", "--", ":(top,literal)"+repoPath+"/")
	var bound *BoundError
	if errors.As(err, &bound) && bound.Kind == BoundStdout {
		return true, nil
	}
	if err != nil {
		return false, indexReadError(repoPath, err)
	}
	paths := SplitNULPaths(output)
	if slices.ContainsFunc(paths, func(path string) bool { return !strings.HasPrefix(path, repoPath+"/") }) {
		return false, indexReadError(repoPath, errIndexEntryForAnotherPath)
	}
	return len(paths) > 0, nil
}

func indexReadError(repoPath string, cause error) error {
	return &OperationalError{Message: fmt.Sprintf("coach codesignal: the index entry for %q could not be read: %s", repoPath, cause), Cause: cause}
}
