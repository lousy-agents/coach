package codesignalcli

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// WorktreeStatus is one read of the working-tree status: the entries git
// listed, or the error that stopped the read. A scan takes it once, before it
// writes anything, and hands the value to every part of the report that
// describes the working tree. Those parts cannot then disagree about the tree,
// and a file the scan itself writes is never counted as the customer's.
type WorktreeStatus struct {
	Entries []gitrepo.WorktreeEntry
	// Warnings are what git said on stderr under --worktree while it succeeded:
	// the directories it could not open, which are absent from Entries, and any
	// warning the scan does not recognize.
	Warnings gitrepo.StatusWarnings
	// SubmodulesSkippedBy is the version of the git that read the status without
	// looking into submodules, because it cannot be told not to fetch; empty
	// when the status looked into them.
	SubmodulesSkippedBy string
	Err                 error
}

// ReadWorktreeStatus reads the status the way the scan will use it: hardened
// against the repository's own configuration under --worktree, and with the
// caller's git environment otherwise, as a committed scan always has.
func ReadWorktreeStatus(dir string, worktree bool) WorktreeStatus {
	if !worktree {
		entries, err := gitrepo.CommittedWorktreeStatus(dir)
		return WorktreeStatus{Entries: entries, Err: err}
	}
	read, err := gitrepo.ReadWorktreeStatus(dir)
	if err != nil {
		err = fmt.Errorf("coach: listing working tree changes: %w", err)
	}
	return WorktreeStatus{Entries: read.Entries, Warnings: read.Warnings, SubmodulesSkippedBy: read.SubmodulesSkippedBy, Err: err}
}
