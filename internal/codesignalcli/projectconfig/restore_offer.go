package projectconfig

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// restoreOffer is what is added to a missing policy that is committed at the
// analyzed revision: the way out, if any, and the sentence that says so. Each
// one withholds the restore command when running it would overwrite something
// or recreate what the scan refuses, and none offers a new policy, because the
// policy is committed already.
type restoreOffer struct {
	remedy projectreadiness.PolicyRemedy
	// cause is, for an offer that names no remedy, why none is named, as the stable word the
	// readiness result carries. It is set beside the text and the remedy so that all three come
	// from one decision. An offer that names a remedy has none.
	cause projectreadiness.PolicyCause
	text  string
}

// restorableOffer brings the committed policy back with the restore command.
func restorableOffer(revision, repoPath string) restoreOffer {
	return restoreOffer{remedy: projectreadiness.RemedyRestore, text: RestoreSentence(projectreadiness.RemedyRestore, revision, repoPath, AsAnOffer)}
}

// restorableOverEmptyEntryOffer is for a committed policy whose index entry holds
// no content (git add --intent-to-add, or an emptied file staged). The restore
// replaces that entry with the committed one and loses nothing, and the message
// says so rather than calling an empty entry an edit.
func restorableOverEmptyEntryOffer(revision, repoPath string) restoreOffer {
	return restoreOffer{remedy: projectreadiness.RemedyRestoreOverEmptyEntry, text: RestoreSentence(projectreadiness.RemedyRestoreOverEmptyEntry, revision, repoPath, AsAnOffer)}
}

// undecidableOffer is for a policy whose git questions went unanswered. Offering
// a restore could overwrite, and offering authoring could replace a reviewed
// policy, so neither is offered. It says which question failed and what git said.
func undecidableOffer(failure *gitFailure) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseGitUnreadable,
		text:   ". Coach could not check " + failure.describe() + ", so it offers neither a restore nor a new policy; resolve the git error and run again",
	}
}

// stagedEditOffer is for a committed policy whose index entry holds an edit. The
// restore command resets the index entry as well, which would discard the edit,
// and so would restoring from git after a blocking file is moved. The edit is
// kept by restoring from the index: git restore with no source writes what the
// index holds, the staged content and not the committed copy. It is dropped by
// the restore from the revision, which brings the committed copy back to both. A
// file in the way is moved before either. Both are printed, so the text cannot say
// that none is offered: the restore from the revision is withheld as the default and
// named as the way to drop the edit.
func stagedEditOffer(revision, repoPath string, blocked bool) restoreOffer {
	keep, drop := RestoreFromIndexAdvice(repoPath), restoreAdvice(revision, repoPath)
	if blocked {
		keep, drop = "move the file that blocks the path, then "+keep, "move the file that blocks the path, then "+drop
	}
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseStagedEdit,
		text: ". It is committed at the analyzed revision, and the index holds a staged edit to it that a restore from the revision would discard, so it is not offered as the default; review the staged edit with git diff --cached, then run again; " +
			"to keep it as it is, which writes the staged content to disk and not the committed copy, " + keep + "; to drop it and bring back the committed copy instead, " + drop,
	}
}

// mergeConflictOffer is for a committed policy whose index entry is unmerged.
// There is no staged edit to review, and a restore would discard the conflict.
func mergeConflictOffer(repoPath string, sides conflictSides, blocked bool) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseMergeConflict,
		text:   ". It is committed at the analyzed revision, and the index holds an unresolved merge conflict for it that a restore would discard, so none is offered; " + resolveConflictAdvice(repoPath, sides, blocked) + ", then run again",
	}
}

// skipWorktreeOffer is for a committed policy whose index entry carries a
// skip-worktree mark but no sparse checkout is on: restoring the file fails
// until the mark is cleared.
func skipWorktreeOffer(repoPath string) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseSkipWorktree,
		text:   ". It is committed at the analyzed revision, but its index entry is marked skip-worktree, so a restore cannot bring it back; " + clearSkipWorktreeAdvice(repoPath) + ", then run again",
	}
}

// stagedOnlyOffer is for a policy that is not committed but whose index entry
// still holds a copy. A new policy would replace work the user has staged, and
// restoring the file from the index overwrites nothing. The
// command is the tail of the text, because a caller that pastes it takes
// everything after "restore it with ".
func stagedOnlyOffer(repoPath string, blocked bool) restoreOffer {
	if blocked {
		return restoreOffer{
			remedy: projectreadiness.RemedyNone,
			cause:  projectreadiness.CauseStagedCopy,
			text:   ". It is not committed, but the index holds a staged copy of it, so Coach offers no new policy; move the file that blocks the path, then restore the staged copy from the index",
		}
	}
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseStagedCopy,
		text:   ". It is not committed, but the index holds a staged copy of it, so Coach offers no new policy; to keep the staged copy, " + RestoreFromIndexAdvice(repoPath),
	}
}

// unmergedNotCommittedOffer is for a policy that is not committed and whose index
// entry is unmerged. git restore refuses an unmerged path, so no command is
// printed, and a new policy would replace the conflicting work.
func unmergedNotCommittedOffer(repoPath string, sides conflictSides, blocked bool) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseMergeConflict,
		text:   ". It is not committed, but the index holds an unresolved merge conflict for it, so Coach offers no new policy and a restore cannot bring it back; " + resolveConflictAdvice(repoPath, sides, blocked) + ", then run again",
	}
}

// stagedNonFileOffer is for a policy that is not committed and whose index entry
// is a symbolic link or a submodule entry. Restoring it from the index would
// recreate what the scan refuses, so no command is printed, and a new policy
// would replace the staged entry.
var stagedNonFileOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseStagedNotAFile,
	text:   ". It is not committed, but the index holds it as a symbolic link or another entry that is not a regular file, which a policy cannot be, so there is nothing to restore; name another policy or stage a regular file at this path",
}

// stagedSkipWorktreeOffer is for a policy that is not committed and whose staged
// copy is marked skip-worktree. git restore does not match a path the mark
// hides, so the command would fail; the mark has to be cleared first.
func stagedSkipWorktreeOffer(repoPath string) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseSkipWorktree,
		text:   ". It is not committed, but the index holds a staged copy of it marked skip-worktree, so a restore cannot bring it back and Coach offers no new policy; " + clearSkipWorktreeAdvice(repoPath) + ", then run again",
	}
}

// stagedSparseOffer is for a policy that is not committed and whose staged copy
// the sparse checkout leaves out of the working tree.
var stagedSparseOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseSparseCheckout,
	text:   ". It is not committed, but the index holds a staged copy of it that this sparse checkout leaves out of the working tree, so a restore cannot bring it back and Coach offers no new policy; add its directory to the sparse checkout, then run again",
}

// emptySkipWorktreeOffer is for a policy that is not committed and whose only index
// entry holds no content (git add --intent-to-add, or an emptied file staged) and is
// marked skip-worktree. There is no copy to keep, so it says what the entry is. While
// the mark is on, git add refuses a new file at the path, so a new policy is not offered.
func emptySkipWorktreeOffer(repoPath string) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseSkipWorktree,
		text:   ". It is not committed, and the index holds only an empty entry for it with no content staged, marked skip-worktree, which makes git refuse a new file at the path, so Coach offers no new policy; " + clearSkipWorktreeAdvice(repoPath) + ", then run again",
	}
}

// emptySparseOffer is emptySkipWorktreeOffer for an entry that the sparse checkout
// leaves out of the working tree.
var emptySparseOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseSparseCheckout,
	text:   ". It is not committed, and the index holds only an empty entry for it with no content staged that this sparse checkout leaves out of the working tree, which makes git refuse a new file at the path, so Coach offers no new policy; add its directory to the sparse checkout, then run again",
}

// uncommittedOfferFor is the offer for a policy that is not committed at the
// analyzed revision and that the index holds in a state worth saying, and false
// for every other verdict.
func uncommittedOfferFor(verdict restoreVerdict, repoPath string, sides conflictSides, blocked bool) (restoreOffer, bool) {
	switch verdict {
	case restoreUnmergedNotCommitted:
		return unmergedNotCommittedOffer(repoPath, sides, blocked), true
	case restoreStagedNonFileNotCommitted:
		return stagedNonFileOffer, true
	case restoreStagedSkipWorktreeNotCommitted:
		return stagedSkipWorktreeOffer(repoPath), true
	case restoreStagedSparseNotCommitted:
		return stagedSparseOffer, true
	case restoreEmptySkipWorktreeNotCommitted:
		return emptySkipWorktreeOffer(repoPath), true
	case restoreEmptySparseNotCommitted:
		return emptySparseOffer, true
	case restoreStagedNotCommitted:
		return stagedOnlyOffer(repoPath, blocked), true
	}
	return restoreOffer{}, false
}

// directoryInIndexOffer is for a path the index holds files under. It is a directory
// there, so no file can be restored to it and no policy written at it, whatever the
// commit holds at the path.
var directoryInIndexOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseDirectory,
	text:   ". The index holds files under this path, so it is a directory and not a file; there is nothing to restore and no policy can be written at it, so name a file",
}

// indexLockedOffer is for a restore that would otherwise be offered while a lock
// file stops git from writing the index: the command would fail, so none is
// printed.
var indexLockedOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseIndexLocked,
	text:   ". The index is locked, so a restore would fail and Coach offers neither a restore nor a new policy; wait for the other git process to finish, or remove the lock file if none is running, then run again",
}

// committedLinkOffer is for a policy committed as a symbolic link. Restoring it
// would recreate a link the scan refuses, and a new file would not be the
// committed policy.
var committedLinkOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseCommittedAsLink,
	text:   ". It is committed at the analyzed revision as a symbolic link, which a policy cannot be, so there is nothing to restore; name another policy or commit a regular file at this path",
}

// sparseCheckoutOffer is for a committed policy that the sparse checkout leaves
// out of the working tree. git restore rejects a path the checkout skips, so the
// command would fail, and the way back is to widen the checkout.
var sparseCheckoutOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseSparseCheckout,
	text:   ". It is committed at the analyzed revision, but this sparse checkout leaves it out of the working tree, so a restore cannot bring it back; add its directory to the sparse checkout, then run again",
}

// committedBlockedParentOffer names no restore command, because running one
// before the file is moved would delete the file.
var committedBlockedParentOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseParentNotDirectory,
	text:   ". The policy is committed at the analyzed revision, so after moving the file, restore the policy from git.",
}

// annotated is found with the offer applied.
func annotated(found *ConfigError, offer restoreOffer) *ConfigError {
	result := *found
	result.Remedy = offer.remedy
	result.PolicyCause = offer.cause
	result.Message += offer.text
	return &result
}

// restoreOfferFor names the way back for a policy that is committed at the
// analyzed revision and gone from disk, or reports that there is nothing to add.
// It offers no restore when that would overwrite something or recreate what the
// scan refuses: a file in the way of the path stays in the way, a staged edit
// stays staged even when a file is in the way, and a committed link stays
// unrestored even when a file is in the way; the message says so instead.
func restoreOfferFor(judgment policyJudgment, revision string, found *ConfigError, repoPath string) (restoreOffer, bool) {
	verdict := judgment.verdict
	if found.Kind != KindMissing {
		return submoduleCheckoutOffer(judgment, found, repoPath)
	}
	blocked := found.Missing == MissingBlockedParent
	if judgment.submodule != "" {
		return insideSubmoduleOffer(judgment.submodule, repoPath), true
	}
	if judgment.indexLocked && !blocked {
		return indexLockedOffer, true
	}
	if verdict == restoreNotCommitted && judgment.sparseNewPolicy && !blocked {
		return sparseNewPolicyOffer, true
	}
	if offer, uncommitted := uncommittedOfferFor(verdict, repoPath, judgment.conflictSides, blocked); uncommitted {
		return offer, true
	}
	switch {
	case verdict == restoreNotCommitted, verdict == restoreUndecidable && blocked:
		return restoreOffer{}, false
	case verdict == restoreUndecidable:
		return undecidableOffer(judgment.failure), true
	case verdict == restoreMergeConflict:
		return mergeConflictOffer(repoPath, judgment.conflictSides, blocked), true
	case verdict == restoreDiscardsStagedEdit:
		return stagedEditOffer(revision, repoPath, blocked), true
	case verdict == restoreSkipWorktreeMarked:
		return skipWorktreeOffer(repoPath), true
	case verdict == restoreRecreatesLink:
		return committedLinkOffer, true
	case verdict == restoreSkippedBySparseCheckout:
		return sparseCheckoutOffer, true
	case verdict == restoreDirectoryInIndex:
		return directoryInIndexOffer, true
	case verdict == restoreCommittedInvalid:
		return committedInvalidOffer(judgment.committedInvalid, blocked), true
	case verdict == restoreOverwritesNothing, verdict == restoreOverwritesEmptyEntry:
		return restoreThatLosesNothing(verdict, revision, repoPath, blocked), true
	}
	return unknownVerdictOffer, true
}

// restoreThatLosesNothing is the offer for a committed policy whose restore
// overwrites nothing. A file in the way of the path comes first: the restore would
// write into the place that file holds.
func restoreThatLosesNothing(verdict restoreVerdict, revision, repoPath string, blocked bool) restoreOffer {
	switch {
	case blocked:
		return committedBlockedParentOffer
	case verdict == restoreOverwritesEmptyEntry:
		return restorableOverEmptyEntryOffer(revision, repoPath)
	}
	return restorableOffer(revision, repoPath)
}

// unknownVerdictOffer is for a verdict no arm of restoreOfferFor names. It prints
// no command, because a restore that nothing has judged safe is not offered, and
// offers no new policy either, so adding a verdict without an arm withholds advice
// instead of guessing at it.
var unknownVerdictOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	cause:  projectreadiness.CauseUndetermined,
	text:   ". Coach could not tell what a restore would do here, so it offers neither a restore nor a new policy; resolve the state of the path and run again",
}
