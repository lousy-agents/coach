package projectcheck

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// PolicyChecker is the check Run applies to a policy: one the user named is
// checked on disk, a discovered one at the analyzed revision. Anything that
// reuses the roots Run checked must select the same way.
func PolicyChecker(policy policyref.Ref) func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	if policy.JudgedOnDisk() {
		return checkExplicitPolicy
	}
	return checkDiscoveredPolicy
}

// checkDiscoveredPolicy is checkPolicy for a policy Coach found, which also names a policy the revision
// holds in another case. That is not a gap to author over: the policy is there, at a name Coach does not
// read.
func checkDiscoveredPolicy(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	check, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil || check.Code != projectreadiness.GapPolicyMissing {
		return check, roots, err
	}
	if renamed := projectconfig.WrongCaseAtRevision(runReadinessGit, dir, revision, policyPath); renamed != nil {
		return gapForWrongCase(renamed, policyPath)
	}
	return gapForDiscoveredPolicyOnDisk(dir, revision, policyPath, check)
}

// gapForDiscoveredPolicyOnDisk is the result for a discovered policy that the revision does not hold. When
// the working tree holds something at the path, that decides the gap as it does for a policy named with
// --project-config: authoring refuses a target that exists, so a file that is not committed is committed and
// not written again, and a path the disk cannot use says why. With nothing on disk, missing is the gap and a
// new policy fixes it.
func gapForDiscoveredPolicyOnDisk(dir, revision, policyPath string, missing projectreadiness.Check) (projectreadiness.Check, []string, error) {
	status, err := onDiskStatus(dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if status != nil && status.Missing == projectconfig.MissingAbsent {
		return missing, nil, nil
	}
	return explicitPolicyCheck(status, dir, revision, policyPath)
}

// gapForWrongCase is the result for a discovered policy that is missing where Coach reads it: the gap that
// names the other case it is committed in. When git could not say whether it is, the result is the
// operational error that names the question, which is how every question git could not answer is reported.
func gapForWrongCase(renamed error, policyPath string) (projectreadiness.Check, []string, error) {
	var refusal *projectconfig.ConfigError
	if errors.As(renamed, &refusal) && refusal.GitFailure != nil {
		return projectreadiness.Check{}, nil, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: the policy at %q: %s", policyPath, refusal.GitFailure), Cause: errors.Unwrap(refusal.GitFailure)}
	}
	return policyGapFrom(renamed)
}

// checkPolicy judges the policy at the analyzed revision. An entry that is not a blob is no policy and
// no gap to author over, so it is named for what it is before the path is called missing; a blob
// committed as a link is named by the read that follows.
func checkPolicy(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	exists, err := gitrepo.FileExistsAtRevision(runReadinessGit, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if !exists {
		return policyMissing(dir, revision, policyPath)
	}

	config, err := projectconfig.LoadForReadiness(dir, revision, policyPath)
	if err != nil {
		return policyGapFrom(err)
	}
	return projectreadiness.Check{State: projectreadiness.Pass}, config.Roots, nil
}

// policyMissing is the gap for a path the revision holds no blob at: missing, unless the revision
// holds a directory or another entry there that a policy cannot be.
func policyMissing(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	refusal, err := projectconfig.NonFileAtRevision(runReadinessGit, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if refusal != nil {
		return policyGapFrom(refusal)
	}
	return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: projectreadiness.RemedyAuthor}, nil, nil
}

// causeOrUndetermined is the word for a gap that has no way out: an error that gave none (a usage refusal that
// reached a check) is the state Coach has no other word for, so a consumer never reads a missing cause.
func causeOrUndetermined(cause projectreadiness.PolicyCause) projectreadiness.PolicyCause {
	if cause == "" {
		return projectreadiness.CauseUndetermined
	}
	return cause
}

// policyGapFrom is the policy_invalid gap for a rejection of the committed policy, carrying the way
// out the rejection offers, or the reason it offers none. Any other error is operational.
func policyGapFrom(err error) (projectreadiness.Check, []string, error) {
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) {
		return projectreadiness.Check{}, nil, err
	}
	check := projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid, Remedy: configErr.Remedy}
	return check.WithCause(configErr.Message, causeOrUndetermined(configErr.PolicyCause)), nil, nil
}
