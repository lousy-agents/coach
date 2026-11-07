package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// dirtyRelevanceRoots are the roots an uncommitted path is judged against to
// decide whether the report left it unread. They are the policy's roots, and a
// policy the user named that the working tree refused gave none. Only the
// policy's own path leaves the list then, so the roots of its committed copy
// still say which other uncommitted paths this report did not read. When nothing
// valid is committed at the analyzed revision there are no roots to take, and only
// the policy path and the package and compiler metadata count.
func dirtyRelevanceRoots(dir, revision string, policy policyref.Ref, policyPath string, policyCheck projectreadiness.Check, roots []string) ([]string, error) {
	if !policy.JudgedOnDisk() || policyCheck.State == projectreadiness.Pass {
		return roots, nil
	}
	_, committedRoots, err := checkPolicy(dir, revision, policyPath)
	return committedRoots, err
}
