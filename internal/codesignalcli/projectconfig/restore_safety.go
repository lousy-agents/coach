package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

// restoreVerdict is what a restore of a committed policy that is missing from
// disk would do, judged before one is offered.
type restoreVerdict int

const (
	restoreNotCommitted restoreVerdict = iota
	restoreOverwritesNothing
	restoreDiscardsStagedEdit
	restoreRecreatesLink
	restoreSkippedBySparseCheckout
	restoreUndecidable
)

const regularFileModeMask = "100"

// policyJudgment is what git says about a policy path the working tree could not
// supply. The two facts are separate because the index can hold something nothing
// has read whatever a restore would do: a policy committed as a link, or never
// committed, can have a regular file staged over it.
type policyJudgment struct {
	// verdict is what a restore would do.
	verdict restoreVerdict
	// indexEntryUnread is set when the index holds an entry that differs from the
	// committed policy (any entry at all when nothing is committed), or git could
	// not say, so a report built without that entry does not reflect it.
	indexEntryUnread bool
	// failure is set when a git call that decides the verdict failed, so the
	// message can say which question went unanswered and what git said.
	failure *gitFailure
}

// gitFailure is one git call that failed while a policy path was judged.
type gitFailure struct {
	// question is what the call was meant to settle, worded to follow "could not
	// check".
	question string
	err      error
}

// judgeRestore decides whether the restore command may be offered for repoPath
// and whether the index holds an entry nothing has read. The verdict declines
// when the revision holds something other than a regular file, which the scan
// would refuse again after the restore, when the index holds an edit that
// --staged would throw away, and when a sparse checkout skips the path, which
// git restore rejects. When git cannot answer it declines too, so an operational
// failure is never taken for a clean miss. run is the git runner, a parameter so
// a caller's seam decides what git says.
func judgeRestore(run func(dir string, args ...string) ([]byte, error), dir, revision, repoPath string) policyJudgment {
	committed, found, err := gitrepo.BlobEntryAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: true, failure: &gitFailure{question: "whether it is committed at the analyzed revision", err: err}}
	}
	staged, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		return policyJudgment{verdict: verdictWithoutIndex(committed, found), indexEntryUnread: true, failure: &gitFailure{question: "what the index holds for it", err: err}}
	}
	unread := indexHoldsUnreadEntry(committed, found, staged)
	return policyJudgment{verdict: verdictFor(committed, found, staged, unread), indexEntryUnread: unread}
}

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

func verdictFor(committed gitrepo.TreeEntry, found bool, staged []gitrepo.IndexEntry, unread bool) restoreVerdict {
	switch {
	case !found:
		return restoreNotCommitted
	case !isRegularFileMode(committed.Mode):
		return restoreRecreatesLink
	case unread:
		return restoreDiscardsStagedEdit
	case skippedBySparseCheckout(staged):
		return restoreSkippedBySparseCheckout
	}
	return restoreOverwritesNothing
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
