package gitrepo

import (
	"errors"
	"fmt"
	"strings"
)

// IndexEntry is one entry the index holds for a path. Stage is 0 for a path
// that is not in a merge conflict. SkipWorktree is set for a path a sparse
// checkout leaves out of the working tree. IntentToAdd is set for an entry made
// with git add --intent-to-add, which records that the path will be added and
// holds no content: its blob is the empty blob whatever the file held.
type IndexEntry struct {
	TreeEntry
	Stage        string
	SkipWorktree bool
	IntentToAdd  bool
}

var errUnparseableIndexLine = errors.New("git printed a line this version cannot parse")

// skipWorktreeTag is the status tag ls-files -t prints for a skip-worktree entry.
const skipWorktreeTag = "S"

// intentToAddFlag is the bit of an entry's flags, which ls-files --debug prints
// in hex after the entry, that marks an intent-to-add entry. ls-files -t has no
// tag for it, and the empty blob it carries cannot be told from a staged empty
// file.
const intentToAddFlag = 0x20000000

// IndexEntries lists what the index holds for repoPath, relative to the
// repository root and matched literally. It returns none for a path the index
// does not know, and more than one for a path in a merge conflict. A listing it
// cannot parse is an error: skipping a line would read as an index that holds
// nothing, and skipping flags would read an intent-to-add entry as a staged copy.
func IndexEntries(run Runner, dir, repoPath string) ([]IndexEntry, error) {
	output, err := run(dir, "ls-files", "--stage", "-t", "--debug", "--", ":(top,literal)"+repoPath)
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
