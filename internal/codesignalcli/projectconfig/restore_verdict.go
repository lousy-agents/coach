package projectconfig

import (
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// verdictWithoutIndex is what the commit alone allows when the index cannot be
// listed: a restore of a regular file cannot be shown to overwrite nothing.
func verdictWithoutIndex(committed gitrepo.TreeEntry, found bool) restoreVerdict {
	switch {
	case !found:
		return restoreNotCommitted
	case !isRegularFileMode(committed.Mode):
		return restoreRecreatesLink
	}
	return restoreUndecidable
}

// verdictFor is what a restore of repoPath would do, from what the commit and the
// index hold for it. A path nothing commits is judged by what the index holds
// instead.
func verdictFor(committed gitrepo.TreeEntry, found bool, staged []gitrepo.IndexEntry, unread, sparse bool) restoreVerdict {
	if !found {
		return verdictForUncommitted(staged, sparse)
	}
	switch {
	case !isRegularFileMode(committed.Mode):
		return restoreRecreatesLink
	case holdsUnmergedEntry(staged):
		return restoreMergeConflict
	case unread:
		return restoreDiscardsStagedEdit
	case skippedBySparseCheckout(staged) && sparse:
		return restoreSkippedBySparseCheckout
	case skippedBySparseCheckout(staged):
		return restoreSkipWorktreeMarked
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
		return restoreStagedSparseNotCommitted
	case skippedBySparseCheckout(staged):
		return restoreStagedSkipWorktreeNotCommitted
	}
	return restoreStagedNotCommitted
}

// withoutEmptyEntries drops the entries that hold no content: a regular file at
// stage 0 whose blob is the empty blob. That is what git add --intent-to-add
// stages, and a staged empty file is no policy either. Restoring such an entry
// writes an empty file, and it is not work that a restore of the committed copy,
// or a new policy, would lose. An entry marked skip-worktree stays, because the
// mark alone stops a restore. The listing carries nothing else to go on, which is
// the point: ls-files --stage is stable, and its flags and debug output are not.
func withoutEmptyEntries(entries []gitrepo.IndexEntry) []gitrepo.IndexEntry {
	return slices.DeleteFunc(slices.Clone(entries), holdsNoContent)
}

func holdsNoContent(entry gitrepo.IndexEntry) bool {
	return entry.Stage == "0" && isRegularFileMode(entry.Mode) && gitrepo.IsEmptyBlob(entry.SHA) && !entry.SkipWorktree
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
