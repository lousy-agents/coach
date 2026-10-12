// Package projectcheck runs `coach codesignal --check-project`: it evaluates
// the committed policy, project shape, host runtime, compiler, package
// manager, and dirty worktree, then aggregates them into one readiness result
// with ordered gaps and next actions.
package projectcheck

import (
	"errors"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func Run(dir, revision, configPath string) (*projectreadiness.Result, error) {
	policyPath := configPath
	if policyPath == "" {
		policyPath = projectconfig.DefaultPath
	}

	policy, roots, err := CheckPolicy(dir, revision, policyPath)
	if err != nil {
		return nil, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policy.State == projectreadiness.Pass)
	if err != nil {
		return nil, err
	}
	runtime := tstoolchain.CheckNode()
	node := tstoolchain.NodeCompatibilityMirror(runtime)
	compiler := tstoolchain.ResolveCompiler(dir, roots)
	packageManager := pkgmanager.Check(dir, roots, policy.State == projectreadiness.Pass)

	checks := projectreadiness.Checks{
		ProjectShape:   projectShape,
		Policy:         policy,
		Node:           node,
		Runtime:        runtime,
		Compiler:       compiler,
		PackageManager: packageManager,
	}

	dirty, err := detectRelevantDirtyWorktree(dir, roots, policyPath)
	if err != nil {
		return nil, err
	}

	miseChoices := tstoolchain.EvaluateMiseSetupChoices(dir, roots)
	status, gaps, nextActions, warnings := aggregateReadiness(checks, dirty.RelevantChanges, miseChoices)

	return &projectreadiness.Result{
		SchemaVersion: projectreadiness.SchemaVersion,
		Status:        status,
		Language:      "typescript",
		Revision:      revision,
		DirtyWorktree: dirty,
		Checks:        checks,
		Gaps:          gaps,
		Warnings:      warnings,
		NextActions:   nextActions,
		MiseChoices:   miseChoices,
	}, nil
}

func CheckPolicy(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
	exists, err := gitrepo.FileExistsAtRevision(runReadinessGit, dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Check{}, nil, err
	}
	if !exists {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyMissing}, nil, nil
	}

	config, err := projectconfig.LoadForReadiness(dir, revision, policyPath)
	if err != nil {
		var configErr *projectconfig.ConfigError
		if errors.As(err, &configErr) {
			return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid}, nil, nil
		}
		return projectreadiness.Check{}, nil, err
	}
	return projectreadiness.Check{State: projectreadiness.Pass}, config.Roots, nil
}
