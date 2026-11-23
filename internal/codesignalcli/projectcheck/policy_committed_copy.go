package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
)

// sameAsTheCommittedCopy is whether the bytes read from disk are the revision's blob, or the blob once git's own
// conversions are applied: a checkout that converted line endings is clean to git. The second result is set when the
// blob cannot be read, as it cannot in a partial clone that does not have it.
func sameAsTheCommittedCopy(run gitrepo.Runner, dir, revision string, selection projectconfig.DiskSelection) (same, unreadable bool) {
	committed, err := run(dir, "show", revision+":"+selection.Path)
	if err != nil {
		return false, true
	}
	if projectconfig.Digest(committed) == selection.Digest {
		return true, false
	}
	return projectconfig.HoldsCommittedPolicyConverted(dir, revision, selection.Path), false
}
