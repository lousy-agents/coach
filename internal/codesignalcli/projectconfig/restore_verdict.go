package projectconfig

import (
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// verdictWithoutIndex is what the commit alone allows when the index cannot be
// listed. A commit that holds a link says what a restore would recreate whatever
// the index holds. Otherwise nothing can be shown: a restore of a regular file
// may overwrite staged work, and a path the commit does not hold may have a
// staged copy, so neither a restore nor a new policy is offered over it.
func verdictWithoutIndex(committed gitrepo.TreeEntry, found bool) restoreVerdict {
	if found && !isRegularFileMode(committed.Mode) {
		return restoreRecreatesLink
	}
	return restoreUndecidable
}

// indexFacts is what the index says about the path, read once: the entries that
// matter (the empty ones nothing else makes matter are not among them), whether any
// holds content nothing has read, whether a sparse checkout is on, and whether an
// entry that holds no content was set aside.
type indexFacts struct {
	entries       []gitrepo.IndexEntry
	unread        bool
	sparse        bool
	setAsideEmpty bool
}

// verdictFor is what a restore of repoPath would do, from what the commit and the
// index hold for it. A path nothing commits is judged by what the index holds
// instead. A restore that loses nothing is told apart when it replaces an empty
// entry, because the message says so. A skip-worktree mark, or a sparse checkout that leaves
// the path out, comes before a staged edit: git refuses the restore that keeps the edit for
// such a path and runs the one that drops it, so naming the edit first would leave the
// destructive command as the only one that works. The offer for the mark says that the index
// also holds a staged edit, and the edit is reached once the mark is cleared.
func verdictFor(committed gitrepo.TreeEntry, found bool, facts indexFacts) restoreVerdict {
	if !found {
		return verdictForUncommitted(facts.entries, facts.sparse)
	}
	switch {
	case !isRegularFileMode(committed.Mode):
		return restoreRecreatesLink
	case holdsUnmergedEntry(facts.entries):
		return restoreMergeConflict
	case skippedBySparseCheckout(facts.entries) && facts.sparse:
		return restoreSkippedBySparseCheckout
	case skippedBySparseCheckout(facts.entries):
		return restoreSkipWorktreeMarked
	case facts.unread:
		return restoreDiscardsStagedEdit
	case facts.setAsideEmpty:
		return restoreOverwritesEmptyEntry
	}
	return restoreOverwritesNothing
}

// verdictForUncommitted is the verdict for a policy that is not committed at the
// analyzed revision, which depends only on what the index holds for it. An
// unmerged entry comes before the rest because git restore refuses it, and a
// skip-worktree mark comes before a plain staged copy because git restore does
// not match a path the mark hides.
func verdictForUncommitted(staged []gitrepo.IndexEntry, sparse bool) restoreVerdict {
	switch {
	case len(staged) == 0:
		return restoreNotCommitted
	case holdsUnmergedEntry(staged):
		return restoreUnmergedNotCommitted
	case stagedAsNonRegularFile(staged):
		return restoreStagedNonFileNotCommitted
	case skippedBySparseCheckout(staged) && sparse:
		return markedVerdict(staged, restoreStagedSparseNotCommitted, restoreEmptySparseNotCommitted)
	case skippedBySparseCheckout(staged):
		return markedVerdict(staged, restoreStagedSkipWorktreeNotCommitted, restoreEmptySkipWorktreeNotCommitted)
	}
	return restoreStagedNotCommitted
}

// markedVerdict is withContent when the marked entries include one that holds
// content, and onlyEmpty when every one of them is empty: an entry that holds nothing
// is not a staged copy, and the message should not call it one.
func markedVerdict(staged []gitrepo.IndexEntry, withContent, onlyEmpty restoreVerdict) restoreVerdict {
	if slices.ContainsFunc(staged, func(entry gitrepo.IndexEntry) bool { return !holdsNoContent(entry) }) {
		return withContent
	}
	return onlyEmpty
}

// holdsNoContent is true for an entry that holds no content: a regular file at
// stage 0 whose blob is the empty blob. That is what git add --intent-to-add
// stages, and a staged empty file is no policy either. Restoring such an entry
// writes an empty file, and it is not work that a restore of the committed copy,
// or a new policy, would lose. The listing carries nothing else to go on, which is
// the point: ls-files --stage is stable, and its flags and debug output are not.
func holdsNoContent(entry gitrepo.IndexEntry) bool {
	return entry.Stage == "0" && isRegularFileMode(entry.Mode) && gitrepo.IsEmptyBlob(entry.SHA)
}

// withoutEmptyEntries drops every entry that holds no content. What is left is what
// there is to read, or to lose: an empty entry is never an edit.
func withoutEmptyEntries(entries []gitrepo.IndexEntry) []gitrepo.IndexEntry {
	return slices.DeleteFunc(slices.Clone(entries), holdsNoContent)
}

// withoutUnmarkedEmptyEntries drops the empty entries that nothing else makes
// matter. One marked skip-worktree stays: the mark alone stops a restore, so the
// skip-worktree and sparse-checkout verdicts still have to see it, even though it
// is not an edit.
func withoutUnmarkedEmptyEntries(entries []gitrepo.IndexEntry) []gitrepo.IndexEntry {
	return slices.DeleteFunc(slices.Clone(entries), func(entry gitrepo.IndexEntry) bool {
		return holdsNoContent(entry) && !entry.SkipWorktree
	})
}

// holdsUnmergedEntry is true when the index holds a conflict stage for the path.
func holdsUnmergedEntry(staged []gitrepo.IndexEntry) bool {
	for _, entry := range staged {
		if entry.Stage != "0" {
			return true
		}
	}
	return false
}

// stagedAsNonRegularFile is true when the index holds the path as a symbolic link
// or a submodule entry, which a restore from the index would recreate as
// something the scan refuses.
func stagedAsNonRegularFile(staged []gitrepo.IndexEntry) bool {
	return slices.ContainsFunc(staged, func(entry gitrepo.IndexEntry) bool { return !isRegularFileMode(entry.Mode) })
}

// indexHoldsUnreadEntry is true unless the index holds nothing, or exactly the
// committed entry at stage 0.
func indexHoldsUnreadEntry(committed gitrepo.TreeEntry, found bool, staged []gitrepo.IndexEntry) bool {
	if len(staged) == 0 {
		return false
	}
	return !(found && len(staged) == 1 && staged[0].Stage == "0" && staged[0].TreeEntry == committed)
}

func skippedBySparseCheckout(staged []gitrepo.IndexEntry) bool {
	for _, entry := range staged {
		if entry.SkipWorktree {
			return true
		}
	}
	return false
}

func isRegularFileMode(mode string) bool {
	return len(mode) == 6 && mode[:3] == regularFileModeMask
}
