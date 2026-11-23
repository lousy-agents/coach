package codesignalcli

import (
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// absentPolicyPaths is the policy the revision selects that the working tree does not hold. Git hiding that path leaves
// the scan unable to tell which file the policy on disk would have been, so it counts as in-scope.
func (p *ProjectAnalysis) absentPolicyPaths() []string {
	if p == nil || p.AbsentPolicyPath == "" {
		return nil
	}
	return []string{p.AbsentPolicyPath}
}

func isAbsentPolicy(name string, absentPolicyPaths []string) bool {
	return slices.ContainsFunc(absentPolicyPaths, func(absent string) bool { return gitrepo.SamePath(name, absent) })
}
