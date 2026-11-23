package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// RunAfterPolicyError is Run for a caller that already holds failure, the error loading a named policy produced. When
// the working tree produced that error, readiness reports its verdict instead of reading the working tree a second
// time, so the error and the report cannot disagree about one path. For any other error it is Run.
func RunAfterPolicyError(dir, revision string, policy policyref.Ref, failure *projectconfig.ConfigError) (*projectreadiness.Result, error) {
	result, err := runAfterPolicyErrorBy(readinessGit(), dir, revision, policy, failure)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// runAfterPolicyErrorBy is RunAfterPolicyError with the git that answers given by the caller. It returns the result by
// value; RunAfterPolicyError is the one that returns a pointer.
func runAfterPolicyErrorBy(run gitrepo.Runner, dir, revision string, policy policyref.Ref, failure *projectconfig.ConfigError) (projectreadiness.Result, error) {
	if !policy.JudgedOnDisk() || failure == nil || !failure.FromWorkingTree {
		return runBy(run, tstoolchain.LocateMiseTypescriptInstall, dir, revision, policy)
	}
	judgment := policyJudgment{path: policy.Path, check: gapForWorkingTreeStatus(failure, policy.Path)}
	return evaluate(run, tstoolchain.LocateMiseTypescriptInstall, dir, revision, judgment)
}
