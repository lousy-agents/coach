package codesignalcli

import "github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"

// unselectedPolicySource is the source of a rename or copy when it is a policy
// file the scan did not select: the working tree no longer holds it, and the
// scan never opened it.
func (u unreadInputs) unselectedPolicySource(entry gitrepo.WorktreeEntry) (string, bool) {
	return entry.OldPath, isProjectPolicyFile(entry.OldPath) && !u.hasOwnDiagnostic(entry.OldPath)
}
