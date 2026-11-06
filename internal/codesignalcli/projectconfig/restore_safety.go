package projectconfig

import (
	"errors"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

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
	restoreMergeConflict
	restoreStagedNotCommitted
	restoreSkipWorktreeMarked
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

// newGitFailure keeps git's own error. An OperationalError wraps it in Coach's
// message, which describe would otherwise quote back to the user inside
// Coach's own.
func newGitFailure(question string, err error) *gitFailure {
	var operational *gitrepo.OperationalError
	if errors.As(err, &operational) && operational.Cause != nil {
		err = operational.Cause
	}
	return &gitFailure{question: question, err: err}
}

// judgeRestore decides what a restore of repoPath would do and whether the index
// holds an entry nothing has read. It declines, naming why in the verdict, when
// the revision holds something other than a regular file, which the scan would
// refuse again after the restore; when the index holds an edit or an unmerged
// entry that the command would discard; and when a sparse checkout or a
// skip-worktree mark keeps git restore from writing the file. A staged deletion
// is not an edit a restore loses: it restores the committed blob. When nothing
// commits the path but the index holds a copy, the verdict says so, so a new
// policy is not offered over staged work. When git cannot answer it declines
// too, so an operational failure is never taken for a clean miss. run is the git
// runner, a parameter so a caller's seam decides what git says.
func judgeRestore(run func(dir string, args ...string) ([]byte, error), dir, revision, repoPath string) policyJudgment {
	committed, found, err := gitrepo.BlobEntryAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: true, failure: newGitFailure("whether it is committed at the analyzed revision", err)}
	}
	staged, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		return policyJudgment{verdict: verdictWithoutIndex(committed, found), indexEntryUnread: true, failure: newGitFailure("what the index holds for it", err)}
	}
	unread := indexHoldsUnreadEntry(committed, found, staged)
	return policyJudgment{verdict: verdictFor(committed, found, staged, unread, sparseCheckoutEnabled(run, dir, staged)), indexEntryUnread: unread}
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

func verdictFor(committed gitrepo.TreeEntry, found bool, staged []gitrepo.IndexEntry, unread, sparse bool) restoreVerdict {
	switch {
	case !found && len(staged) > 0:
		return restoreStagedNotCommitted
	case !found:
		return restoreNotCommitted
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

// holdsUnmergedEntry is true when the index holds a conflict stage for the path.
func holdsUnmergedEntry(staged []gitrepo.IndexEntry) bool {
	for _, entry := range staged {
		if entry.Stage != "0" {
			return true
		}
	}
	return false
}

// sparseCheckoutEnabled reports whether core.sparseCheckout is on. It asks git
// only when a skip-worktree entry makes the answer matter, and an unset key,
// which git reports with a failure exit, is taken as off.
func sparseCheckoutEnabled(run func(dir string, args ...string) ([]byte, error), dir string, staged []gitrepo.IndexEntry) bool {
	if !skippedBySparseCheckout(staged) {
		return false
	}
	out, err := run(dir, "config", "--type=bool", "--get", "core.sparseCheckout")
	return err == nil && strings.TrimSpace(string(out)) == "true"
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
