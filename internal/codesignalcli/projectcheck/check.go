// Package projectcheck runs `coach codesignal --check-project`: it evaluates
// the committed policy, project shape, host runtime, compiler, package
// manager, and dirty worktree, then aggregates them into one readiness result
// with ordered gaps and next actions.
package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// Run evaluates the project at revision. configPath is the --project-config the
// user passed and is empty when the policy is to be discovered. The difference
// is not cosmetic: a named policy is checked on disk, and a discovered one at
// the analyzed revision, so a caller holding a resolved path must pass "".
func Run(dir, revision, configPath string) (*projectreadiness.Result, error) {
	result, err := evaluate(dir, revision, configPath, PolicyChecker(configPath))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// evaluate is Run with the policy check chosen by the caller.
func evaluate(dir, revision, configPath string, checkPolicy func(dir, revision, policyPath string) (projectreadiness.Check, []string, error)) (projectreadiness.Result, error) {
	policyPath, extraWarnings, err := resolveCheckPolicy(dir, revision, configPath)
	if err != nil {
		return projectreadiness.Result{}, err
	}

	policy, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Result{}, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policy.State == projectreadiness.Pass)
	if err != nil {
		return projectreadiness.Result{}, err
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

	readFromDisk := ""
	if policy.DecidedOnDisk {
		readFromDisk = policyPath
	}
	dirty, err := detectRelevantDirtyWorktree(dir, roots, policyPath, readFromDisk)
	if err != nil {
		return projectreadiness.Result{}, err
	}

	miseChoices := tstoolchain.EvaluateMiseSetupChoices(dir, roots)
	status, gaps, nextActions, warnings := aggregateReadiness(checks, dirty.RelevantChanges, miseChoices)
	warnings = append(warnings, extraWarnings...)

	return projectreadiness.Result{
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
