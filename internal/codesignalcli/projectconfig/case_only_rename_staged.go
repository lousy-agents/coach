package projectconfig

import (
	"errors"
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// unlessTheRenameIsStaged is the refusal of the case-only rename check, or nil when the index already records the typed
// name. A nil refusal and an unanswered question are the check's own answer and pass through. For a refusal, the index
// is asked what it holds under the typed name: one plain stage 0 file with content is a staged rename, which a commit
// records and the move the refusal offers could not (the index no longer holds the revision's spelling), so the policy
// is left plainly uncommitted. An index that cannot be asked gets no advice, because the move is wrong if the rename is
// staged and the commit is wrong if it is not.
func unlessTheRenameIsStaged(run gitrepo.Runner, dir, repoPath string, refusal error) error {
	var renamed *ConfigError
	if !errors.As(refusal, &renamed) {
		return refusal
	}
	recorded, err := indexRecordsAsTyped(run, dir, repoPath)
	switch {
	case err != nil:
		return newGitFailure("whether the index already records the rename", err)
	case recorded:
		return nil
	}
	return refusal
}

// indexRecordsAsTyped is whether the index already holds the path as typed, as one plain file: the user has staged
// the rename, with git mv, and it only needs committing. Anything else under the typed name keeps the refusal.
func indexRecordsAsTyped(run gitrepo.Runner, dir, repoPath string) (bool, error) {
	held, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(held.Entries, stagedAsAFileWithContent), nil
}

// stagedAsAFileWithContent is a stage 0 regular file that holds something. A conflict, a link and an entry that holds
// nothing (git add --intent-to-add) are not a rename that a commit records.
func stagedAsAFileWithContent(entry gitrepo.IndexEntry) bool {
	return entry.Stage == "0" && isRegularFileMode(entry.Mode) && !holdsNoContent(entry)
}
