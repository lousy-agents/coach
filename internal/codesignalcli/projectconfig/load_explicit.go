package projectconfig

import (
	"encoding/json"
	"errors"
	"fmt"
)

// LoadExplicit is Load for a --project-config the user named. It rejects a
// file that does not exist on disk even when the path is committed at the
// analyzed revision, because an explicit policy never falls back.
func LoadExplicit(dir, revision, repoPath string) (json.RawMessage, error) {
	status, err := ExplicitOnDiskStatus(runProjectConfigGit, dir, revision, repoPath)
	if err != nil {
		return nil, err
	}
	if status != nil {
		return nil, status
	}
	return Load(dir, revision, repoPath)
}

// ExplicitOnDiskStatus is what the working tree says about a policy the user
// named with --project-config. It is nil when the path is a regular file under
// the repository root, and otherwise the *ConfigError that says why not and
// which way out, if any, is safe to offer. For every such error, missing or
// unusable, it also says whether the index holds an edit to the committed policy
// that nothing has read. The error return is an operational
// failure, never a verdict about the policy. run is the git runner that decides
// whether the policy is committed and what the index holds for it.
func ExplicitOnDiskStatus(run func(dir string, args ...string) ([]byte, error), dir, revision, repoPath string) (*ConfigError, error) {
	err := RequireOnDisk(dir, repoPath)
	if err == nil {
		return nil, nil
	}
	var found *ConfigError
	if !errors.As(err, &found) {
		return nil, err
	}
	judgment := judgeRestore(run, dir, revision, repoPath)
	judged := *found
	judged.FromWorkingTree = true
	judged.UnreadStagedEdit = judgment.indexEntryUnread
	if judgment.failure != nil {
		judged.GitFailure = fmt.Errorf("could not check %s", judgment.failure.describe())
	}
	offer, offered := restoreOfferFor(judgment, revision, found, repoPath)
	if !offered {
		return &judged, nil
	}
	return annotated(&judged, offer), nil
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
