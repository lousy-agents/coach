package projectcheck

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// PolicyChecker is the check Run applies to a policy: one the user named is
// checked on disk, a discovered one at the analyzed revision. Anything that
// reuses the roots Run checked must select the same way.
func PolicyChecker(policy policyref.Ref) func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	return policyCheckerBy(readinessGit(), policy)
}

// policyCheckerBy is PolicyChecker with the git that answers given by the caller.
func policyCheckerBy(run gitrepo.Runner, policy policyref.Ref) func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	if policy.JudgedOnDisk() {
		return func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
			return checkExplicitPolicy(run, dir, revision, policyPath)
		}
	}
	return func(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
		return checkDiscoveredPolicy(run, dir, revision, policyPath)
	}
}

// checkPolicy judges the policy at the analyzed revision. An entry that is not a blob is no policy and
// no gap to author over, so it is named for what it is before the path is called missing; a blob
// committed as a link is named by the read that follows.
func checkPolicy(run gitrepo.Runner, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	exists, err := gitrepo.FileExistsAtRevision(run, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if !exists {
		return policyMissing(run, dir, revision, policyPath)
	}

	config, err := projectconfig.LoadForReadiness(dir, revision, policyPath)
	if err != nil {
		return policyGapFrom(err)
	}
	return projectreadiness.Check{State: projectreadiness.Pass}, config.Roots, nil
}

// policyMissing is the gap for a path the revision holds no blob at: missing, unless the revision
// holds a directory or another entry there that a policy cannot be.
func policyMissing(run gitrepo.Runner, dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	refusal, err := projectconfig.NonFileAtRevision(run, dir, revision, policyPath)
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
