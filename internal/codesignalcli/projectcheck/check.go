// Package projectcheck runs `coach codesignal --check-project`: it evaluates
// the committed policy, project shape, host runtime, compiler, package
// manager, and dirty worktree, then aggregates them into one readiness result
// with ordered gaps and next actions.
package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// Run evaluates the project at revision. A policy the user named is checked on
// disk at its path; a discovered one is selected again by Discover and checked
// at the analyzed revision, so a caller holding a discovered policy may pass it
// as it is.
func Run(dir, revision string, policy policyref.Ref) (*projectreadiness.Result, error) {
	result, err := evaluate(dir, revision, policy, PolicyChecker(policy))
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// evaluate is Run with the policy check chosen by the caller.
func evaluate(dir, revision string, policy policyref.Ref, check func(dir, revision, policyPath string) (projectreadiness.Check, []string, error)) (projectreadiness.Result, error) {
	policyPath, extraWarnings, err := resolveCheckPolicy(dir, revision, policy)
	if err != nil {
		return projectreadiness.Result{}, err
	}

	policyCheck, roots, err := check(dir, revision, policyPath)
	if err != nil {
		return projectreadiness.Result{}, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policyCheck.State == projectreadiness.Pass)
	if err != nil {
		return projectreadiness.Result{}, err
	}
	runtime := tstoolchain.CheckNode()
	node := tstoolchain.NodeCompatibilityMirror(runtime)
	compiler := tstoolchain.ResolveCompiler(dir, roots)
	packageManager := pkgmanager.Check(dir, roots, policyCheck.State == projectreadiness.Pass)

	checks := projectreadiness.Checks{
		ProjectShape:   projectShape,
		Policy:         policyCheck,
		Node:           node,
		Runtime:        runtime,
		Compiler:       compiler,
		PackageManager: packageManager,
	}

	readFromDisk := ""
	if policyCheck.DecidedOnDisk {
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
