package projectcheck

import "github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"

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
