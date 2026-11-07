package projectconfig

import (
	"errors"

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
	restoreUnmergedNotCommitted
	restoreStagedNonFileNotCommitted
	restoreStagedSkipWorktreeNotCommitted
	restoreStagedSparseNotCommitted
	restoreOverwritesEmptyEntry
	restoreDirectoryInIndex
	restoreEmptySkipWorktreeNotCommitted
	restoreEmptySparseNotCommitted
	restoreCommittedInvalid
	// restoreVerdictCount is the number of verdicts above, for a test that walks all of them.
	restoreVerdictCount
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
	// submodule is the submodule the policy path lies inside, if any. Nothing there can be
	// restored or committed in this repository, so it is decided before anything else.
	submodule string
	// committedInvalid is, for restoreCommittedInvalid, why the committed copy is not a valid policy.
	committedInvalid string
	// sparseNewPolicy is set when nothing commits the path, it is in a repository with a sparse
	// checkout, and its directory is not on disk: a new policy there may need git add --sparse.
	sparseNewPolicy bool
	// indexLocked is set when a restore command would otherwise be offered while a lock file stops git
	// from writing the index: running it would fail, so none is offered.
	indexLocked bool
}

// gitFailure is one git call that failed while a policy path was judged.
type gitFailure struct {
	// question is what the call was meant to settle, worded to follow "could not
	// check".
	question string
	err      error
	// words replace what git said when Coach knows what the failure means and git's
	// own words, here a bare exit status, would not tell the user.
	words string
}

// newGitFailure keeps git's own error. An OperationalError wraps it in Coach's
// message, which describe would otherwise quote back to the user inside
// Coach's own.
func newGitFailure(question string, err error) *gitFailure {
	var operational *gitrepo.OperationalError
	if errors.As(err, &operational) && operational.Cause != nil {
		err = operational.Cause
	}
	var read *gitrepo.BlobReadError
	if errors.As(err, &read) {
		err = read.Cause
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
// policy is not offered over staged work; an unmerged copy is told apart, because
// git restore refuses it. When git cannot answer it declines
// too, so an operational failure is never taken for a clean miss. run is the git
// runner, a parameter so a caller's seam decides what git says.
func judgeRestore(run gitrepo.Runner, dir, revision, repoPath string) policyJudgment {
	if judged, decided := judgeInsideASubmodule(run, dir, repoPath); decided {
		return judged
	}
	committed, found, err := gitrepo.BlobEntryAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: true, failure: blobCheckFailure(run, dir, revision, repoPath, err)}
	}
	indexed, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		return policyJudgment{verdict: verdictWithoutIndex(committed, found), indexEntryUnread: true, failure: newGitFailure("what the index holds for it", err)}
	}
	if indexed.HoldsBelow {
		return policyJudgment{verdict: restoreDirectoryInIndex}
	}
	listed := indexed.Entries
	staged := withoutUnmarkedEmptyEntries(listed)
	unread := indexHoldsUnreadEntry(committed, found, withoutEmptyEntries(listed))
	sparse, err := sparseCheckoutEnabled(run, dir, staged)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: unread, failure: newGitFailure("whether a sparse checkout is on", err)}
	}
	facts := indexFacts{entries: staged, unread: unread, sparse: sparse, setAsideEmpty: len(staged) < len(listed)}
	verdict := verdictFor(committed, found, facts)
	if restoresFromTheCommit(verdict) {
		if judged, decided := judgeCommittedCopy(run, dir, revision, repoPath); decided {
			return judged
		}
	}
	if !offersRestoreCommand(verdict) {
		return policyJudgment{verdict: verdict, indexEntryUnread: unread, sparseNewPolicy: verdict == restoreNotCommitted && newPolicyMayNeedSparse(run, dir, repoPath)}
	}
	locked, err := indexIsLocked(run, dir)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: unread, failure: newGitFailure("whether the index is locked", err)}
	}
	return policyJudgment{verdict: verdict, indexEntryUnread: unread, indexLocked: locked}
}

// offersRestoreCommand is whether a verdict ends in a printed restore command, which is what a
// locked index makes fail.
func offersRestoreCommand(verdict restoreVerdict) bool {
	return verdict == restoreOverwritesNothing || verdict == restoreOverwritesEmptyEntry || verdict == restoreStagedNotCommitted
}
