package projectconfig

import "github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"

// restoreOffer is what is added to a missing policy that is committed at the
// analyzed revision: the way out, if any, and the sentence that says so. Each
// one withholds the restore command when running it would overwrite something
// or recreate what the scan refuses, and none offers a new policy, because the
// policy is committed already.
type restoreOffer struct {
	remedy projectreadiness.PolicyRemedy
	text   string
}

// restorableOffer brings the committed policy back with the restore command.
func restorableOffer(revision, repoPath string) restoreOffer {
	return restoreOffer{remedy: projectreadiness.RemedyRestore, text: ". It is committed at the analyzed revision: " + RestoreAdvice(revision, repoPath)}
}

// undecidableOffer is for a policy whose git questions went unanswered. Offering
// a restore could overwrite, and offering authoring could replace a reviewed
// policy, so neither is offered. It says which question failed and what git said.
func undecidableOffer(failure *gitFailure) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		text:   ". Coach could not check " + failure.describe() + ", so it offers neither a restore nor a new policy; resolve the git error and run again",
	}
}

// stagedEditOffer is for a committed policy whose index entry holds an edit. The
// restore command resets the index entry as well, which would discard the edit,
// and so would restoring from git after a blocking file is moved.
var stagedEditOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision, and the index holds a staged edit to it that a restore would discard, so none is offered; review the staged edit with git diff --cached, keep what you want, then run again",
}

// mergeConflictOffer is for a committed policy whose index entry is unmerged.
// There is no staged edit to review, and a restore would discard the conflict.
var mergeConflictOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision, and the index holds an unresolved merge conflict for it that a restore would discard, so none is offered; resolve the conflict with git status, then run again",
}

// skipWorktreeOffer is for a committed policy whose index entry carries a
// skip-worktree mark but no sparse checkout is on: restoring the file fails
// until the mark is cleared.
func skipWorktreeOffer(repoPath string) restoreOffer {
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
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
			text:   ". It is not committed, but the index holds a staged copy of it, so Coach offers no new policy; move the file that blocks the path, then restore the staged copy from the index",
		}
	}
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		text:   ". It is not committed, but the index holds a staged copy of it, so Coach offers no new policy; to keep the staged copy, " + RestoreFromIndexAdvice(repoPath),
	}
}

// committedLinkOffer is for a policy committed as a symbolic link. Restoring it
// would recreate a link the scan refuses, and a new file would not be the
// committed policy.
var committedLinkOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision as a symbolic link, which a policy cannot be, so there is nothing to restore; name another policy or commit a regular file at this path",
}

// sparseCheckoutOffer is for a committed policy that the sparse checkout leaves
// out of the working tree. git restore rejects a path the checkout skips, so the
// command would fail, and the way back is to widen the checkout.
var sparseCheckoutOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision, but this sparse checkout leaves it out of the working tree, so a restore cannot bring it back; add its directory to the sparse checkout, then run again",
}

// committedBlockedParentOffer names no restore command, because running one
// before the file is moved would delete the file.
var committedBlockedParentOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". The policy is committed at the analyzed revision, so after moving the file, restore the policy from git.",
}

// annotated is found with the offer applied.
func annotated(found *ConfigError, offer restoreOffer) *ConfigError {
	result := *found
	result.Remedy = offer.remedy
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
		return restoreOffer{}, false
	}
	blocked := found.Cause == CauseBlockedParent
	switch {
	case verdict == restoreNotCommitted, verdict == restoreUndecidable && blocked:
		return restoreOffer{}, false
	case verdict == restoreUndecidable:
		return undecidableOffer(judgment.failure), true
	case verdict == restoreStagedNotCommitted:
		return stagedOnlyOffer(repoPath, blocked), true
	case verdict == restoreMergeConflict:
		return mergeConflictOffer, true
	case verdict == restoreDiscardsStagedEdit:
		return stagedEditOffer, true
	case verdict == restoreSkipWorktreeMarked:
		return skipWorktreeOffer(repoPath), true
	case verdict == restoreRecreatesLink:
		return committedLinkOffer, true
	case verdict == restoreSkippedBySparseCheckout:
		return sparseCheckoutOffer, true
	case blocked:
		return committedBlockedParentOffer, true
	}
	return restorableOffer(revision, repoPath), true
}
