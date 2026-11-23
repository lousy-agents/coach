package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// dirtyRelevanceRoots are the roots an uncommitted path is judged against to
// decide whether the report left it unread. They are the roots of the policy the
// check read. A policy the working tree gave no usable copy of gave none, so the
// roots of its committed copy still say which other uncommitted paths this report
// did not read. When nothing valid is committed at the analyzed revision there are
// no roots to take, and only the package and compiler metadata count.
func dirtyRelevanceRoots(run gitrepo.Runner, dir, revision string, judgment policyJudgment) ([]string, error) {
	if judgment.check.State == projectreadiness.Pass {
		return judgment.roots, nil
	}
	_, committedRoots, err := checkPolicy(run, dir, revision, judgment.path)
	return committedRoots, err
}
