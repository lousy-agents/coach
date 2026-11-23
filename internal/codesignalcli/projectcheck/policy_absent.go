package projectcheck

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// judgeAbsentPolicy is the gap for a policy with no usable file on disk: the entry there that cannot be a policy, or
// nothing. The revision and the index then decide which way out is safe to offer, as they do for a scan.
func judgeAbsentPolicy(run gitrepo.Runner, dir, revision string, policy policyref.Ref) (policyJudgment, error) {
	policyPath, warnings, err := resolveCheckPolicy(dir, revision, policy)
	if err != nil {
		return policyJudgment{}, err
	}
	status, err := onDiskStatus(run, dir, revision, policyPath)
	if err != nil {
		return policyJudgment{}, err
	}
	if status == nil {
		return policyJudgment{}, errors.New("coach codesignal --check-project: the policy file appeared while it was being checked; run the check again")
	}
	check := wordedForThePolicy(gapForWorkingTreeStatus(namedForWhatIsOnDisk(dir, status, policyPath), policyPath), policy, policyPath)
	return policyJudgment{path: policyPath, check: check, warnings: warnings}, nil
}

// namedForWhatIsOnDisk is status with the cause the disk gives where the cause is one the check no longer names: an
// entry on disk that cannot be the policy is a symbolic link, not a regular file, or under a file, whatever the index or
// the revision hold for the path. A path with nothing on disk keeps status.
func namedForWhatIsOnDisk(dir string, status *projectconfig.ConfigError, policyPath string) *projectconfig.ConfigError {
	if causeTheCheckNames(status.PolicyCause) {
		return status
	}
	refusal := projectconfig.UnusableEntryOnDisk(dir, policyPath)
	if refusal == nil {
		return status
	}
	cause, known := causeOfDiskRefusal[refusal.Reason]
	if !known {
		return status
	}
	named := *status
	named.PolicyCause = cause
	named.Message = fmt.Sprintf("coach codesignal: %q cannot be the policy on disk: %s", policyPath, refusal.Reason)
	return &named
}

// causeOfDiskRefusal is the cause for each reason projectconfig.UnusableEntryOnDisk gives, which are a fixed set.
var causeOfDiskRefusal = map[string]projectreadiness.PolicyCause{
	"a symbolic link":                             projectreadiness.CauseSymbolicLink,
	"not a regular file":                          projectreadiness.CauseNotARegularFile,
	"a file stands where its directory should be": projectreadiness.CauseParentNotDirectory,
}

// gapForWorkingTreeStatus is the gap for what the working tree says of a policy path, carrying the way out it offers
// or the reason it offers none. Where the index or the revision holds work for the path, no restore from the revision
// is named, because it would discard that work; the cause is undetermined and the message names no command.
func gapForWorkingTreeStatus(status *projectconfig.ConfigError, policyPath string) projectreadiness.Check {
	code := projectreadiness.GapPolicyInvalid
	if status.Kind == projectconfig.KindMissing {
		code = projectreadiness.GapPolicyMissing
	}
	check := projectreadiness.Check{State: projectreadiness.Fail, Code: code, Remedy: status.Remedy}
	if !causeTheCheckNames(status.PolicyCause) {
		return check.WithCause(heldAtThePathText(policyPath), projectreadiness.CauseUndetermined)
	}
	return check.WithCause(status.Message, causeOrUndetermined(status.PolicyCause))
}

// causeTheCheckNames is whether the check names the cause a path that is not usable on disk is given. The causes that
// say what the index or the revision holds for it are disclosed as warnings when a file is on disk, and are
// undetermined when none is.
func causeTheCheckNames(cause projectreadiness.PolicyCause) bool {
	switch cause {
	case "",
		projectreadiness.CauseDirectory,
		projectreadiness.CauseSymbolicLink,
		projectreadiness.CauseNotARegularFile,
		projectreadiness.CauseUnreadableOnDisk,
		projectreadiness.CauseParentNotDirectory,
		projectreadiness.CauseNameCaseDiffers,
		projectreadiness.CauseInSubmodule,
		projectreadiness.CauseUndetermined,
		projectreadiness.CauseSparseCheckout,
		projectreadiness.CauseSkipWorktree:
		return true
	}
	return false
}

func heldAtThePathText(policyPath string) string {
	return "coach codesignal: " + policyPath + " is not on disk, and the index or the analyzed revision holds an entry for it that a restore from the revision would discard or recreate as something a policy cannot be; resolve it with git, then check again"
}
