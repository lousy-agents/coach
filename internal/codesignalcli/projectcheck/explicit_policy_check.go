package projectcheck

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// CheckExplicitPolicy is CheckPolicy for a path the user named with
// --project-config. A scan refuses such a path unless it is a regular file on
// disk, so readiness must not pass the committed copy of one that is not.
func CheckExplicitPolicy(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	status, err := projectconfig.ExplicitOnDiskStatus(runReadinessGit, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if status != nil && status.GitFailure != nil {
		return projectreadiness.Check{}, nil, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: --project-config %q: %s", policyPath, status.GitFailure)}
	}
	return explicitPolicyCheck(status, dir, revision, policyPath)
}

// RunAfterPolicyError is Run for a caller that already holds failure, the error
// loading policy produced. When the working tree produced that error, readiness
// reports its verdict instead of reading the working tree a second time, so the
// error and the report cannot disagree about one path. For a discovered policy,
// and for an error the analyzed revision produced, it is Run.
func RunAfterPolicyError(dir, revision string, policy policyref.Ref, failure *projectconfig.ConfigError) (*projectreadiness.Result, error) {
	if !policy.JudgedOnDisk() {
		return Run(dir, revision, policy)
	}
	check := func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
		if failure != nil && failure.FromWorkingTree {
			return explicitPolicyCheck(failure, dir, revision, policyPath)
		}
		return explicitPolicyCheck(nil, dir, revision, policyPath)
	}
	result, err := evaluate(dir, revision, policy, check)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// explicitPolicyCheck is the check for a named policy given what the working
// tree said about it: nil for a regular file, which the analyzed revision then
// judges, and otherwise the error that says why not.
func explicitPolicyCheck(status *projectconfig.ConfigError, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	if status == nil {
		return checkPolicyOnDisk(dir, revision, policyPath)
	}
	code := projectreadiness.GapPolicyInvalid
	if status.Kind == projectconfig.KindMissing {
		code = projectreadiness.GapPolicyMissing
	}
	return projectreadiness.Check{State: projectreadiness.Fail, Code: code, DecidedOnDisk: !status.UnreadStagedEdit, Remedy: status.Remedy}, nil, nil
}

// checkPolicyOnDisk is CheckPolicy for a path that is a regular file on disk. A
// policy missing at the revision is there on disk, so a new policy cannot be
// written at the path and committing the file is the way out.
func checkPolicyOnDisk(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	check, roots, err := CheckPolicy(dir, revision, policyPath)
	if err == nil && check.Code == projectreadiness.GapPolicyMissing {
		check.Remedy = projectreadiness.RemedyNone
	}
	return check, roots, err
}
