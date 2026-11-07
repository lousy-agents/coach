package projectcheck

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// checkExplicitPolicy is checkPolicy for a path the user named with
// --project-config. A scan refuses such a path unless it is a regular file on
// disk, so readiness must not pass the committed copy of one that is not.
func checkExplicitPolicy(run gitrepo.Runner, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	status, err := onDiskStatus(run, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	return explicitPolicyCheck(run, status, dir, revision, policyPath)
}

// RunAfterPolicyError is Run for a caller that already holds failure, the error
// loading policy produced. When the working tree produced that error, readiness
// reports its verdict instead of reading the working tree a second time, so the
// error and the report cannot disagree about one path. For a discovered policy,
// and for an error the analyzed revision produced, it is Run.
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
	if !policy.JudgedOnDisk() {
		return runBy(run, tstoolchain.LocateMiseTypescriptInstall, dir, revision, policy)
	}
	check := func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
		if failure != nil && failure.FromWorkingTree {
			return explicitPolicyCheck(run, failure, dir, revision, policyPath)
		}
		return explicitPolicyCheck(run, nil, dir, revision, policyPath)
	}
	return evaluate(run, tstoolchain.LocateMiseTypescriptInstall, dir, revision, policy, check)
}

// explicitPolicyCheck is the check for a named policy given what the working
// tree said about it: nil for a regular file, which the analyzed revision then
// judges, and otherwise the error that says why not.
func explicitPolicyCheck(run gitrepo.Runner, status *projectconfig.ConfigError, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	if status == nil {
		return checkPolicyOnDisk(run, dir, revision, policyPath)
	}
	code := projectreadiness.GapPolicyInvalid
	if status.Kind == projectconfig.KindMissing {
		code = projectreadiness.GapPolicyMissing
	}
	return projectreadiness.Check{State: projectreadiness.Fail, Code: code, DecidedOnDisk: !status.UnreadStagedEdit, Remedy: status.Remedy}.WithCause(status.Message, status.PolicyCause), nil, nil
}

// checkPolicyOnDisk is checkPolicy for a path that is a regular file on disk. A
// policy missing at the revision is there on disk, so a new policy cannot be
// written at the path and committing the file is the way out.
func checkPolicyOnDisk(run gitrepo.Runner, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	check, roots, err := checkPolicy(run, dir, revision, policyPath)
	if err == nil && check.Code == projectreadiness.GapPolicyMissing {
		check.Remedy = projectreadiness.RemedyNone
		message, cause, unanswered := projectconfig.UncommittedPolicy(run, dir, revision, policyPath)
		if unanswered != nil {
			return projectreadiness.Check{}, nil, unansweredQuestion(policyPath, unanswered)
		}
		check = check.WithCause(message, cause)
		// The file's presence made the gap this one and not a missing policy to author, so the check read it.
		check.DecidedOnDisk = true
	}
	return check, roots, err
}

// unansweredQuestion is a question about the policy that git could not answer, as readiness reports it:
// an operational error that keeps git's own error as its cause, whichever state the policy was in, so a
// failure of git never reads as a verdict about the policy.
func unansweredQuestion(policyPath string, failure error) error {
	return &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: --project-config %q: %s", policyPath, failure), Cause: errors.Unwrap(failure)}
}
