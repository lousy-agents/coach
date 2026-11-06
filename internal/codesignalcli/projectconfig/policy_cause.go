package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"

// causeOfVerdict is the word for each verdict whose offer names no remedy. The
// verdicts whose offer is a restore carry no cause, and a restore withheld only
// because a file is in the way of the path is told apart in offerCause.
var causeOfVerdict = map[restoreVerdict]projectreadiness.PolicyCause{
	restoreDiscardsStagedEdit:             projectreadiness.CauseStagedEdit,
	restoreRecreatesLink:                  projectreadiness.CauseCommittedAsLink,
	restoreSkippedBySparseCheckout:        projectreadiness.CauseSparseCheckout,
	restoreUndecidable:                    projectreadiness.CauseGitUnreadable,
	restoreMergeConflict:                  projectreadiness.CauseMergeConflict,
	restoreSkipWorktreeMarked:             projectreadiness.CauseSkipWorktree,
	restoreStagedNotCommitted:             projectreadiness.CauseStagedCopy,
	restoreUnmergedNotCommitted:           projectreadiness.CauseMergeConflict,
	restoreStagedNonFileNotCommitted:      projectreadiness.CauseStagedNotAFile,
	restoreStagedSkipWorktreeNotCommitted: projectreadiness.CauseSkipWorktree,
	restoreStagedSparseNotCommitted:       projectreadiness.CauseSparseCheckout,
	restoreDirectoryInIndex:               projectreadiness.CauseDirectory,
}

// offerCause is why an offer that names no remedy names none. A locked index comes
// first, because it withholds a restore whatever the verdict says; a restore that
// would overwrite nothing is withheld only when a file is in the way of the path.
func offerCause(judgment policyJudgment, blocked bool) projectreadiness.PolicyCause {
	if judgment.indexLocked && !blocked {
		return projectreadiness.CauseIndexLocked
	}
	if blocked && offersRestoreCommand(judgment.verdict) {
		return projectreadiness.CauseParentNotDirectory
	}
	if cause, known := causeOfVerdict[judgment.verdict]; known {
		return cause
	}
	return projectreadiness.CauseUndetermined
}
