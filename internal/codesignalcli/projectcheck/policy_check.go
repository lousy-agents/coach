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
	if policy.JudgedOnDisk() {
		return CheckExplicitPolicy
	}
	return CheckPolicy
}

func CheckPolicy(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	exists, err := gitrepo.FileExistsAtRevision(runReadinessGit, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if !exists {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing, Remedy: projectreadiness.RemedyAuthor}, nil, nil
	}

	config, err := projectconfig.LoadForReadiness(dir, revision, policyPath)
	if err != nil {
		var configErr *projectconfig.ConfigError
		if errors.As(err, &configErr) {
			return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid, Remedy: projectreadiness.RemedyAuthor}, nil, nil
		}
		return projectreadiness.Check{}, nil, err
	}
	return projectreadiness.Check{State: projectreadiness.Pass}, config.Roots, nil
}
