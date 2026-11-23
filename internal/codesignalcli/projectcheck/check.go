// Package projectcheck runs `coach codesignal --check-project`: it evaluates
// the committed policy, project shape, host runtime, compiler, package
// manager, and dirty worktree, then aggregates them into one readiness result
// with ordered gaps and next actions.
package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/pkgmanager"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// Run evaluates the project at revision. The policy is judged from the working
// tree: the file the user named, or the one discovery finds there. A caller
// holding a discovered policy may pass it as it is.
func Run(dir, revision string, policy policyref.Ref) (*projectreadiness.Result, error) {
	result, err := RunWithLocator(tstoolchain.LocateMiseTypescriptInstall, dir, revision, policy)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// RunWithLocator is Run with the locator that finds a compiler mise installed given by the caller. The setup that
// installs a compiler rechecks readiness through the locator it installed with, so the recheck looks where the install
// put the compiler. It returns the result by value, as evaluate does; Run is the one that returns a pointer.
func RunWithLocator(locate tstoolchain.MiseLocator, dir, revision string, policy policyref.Ref) (projectreadiness.Result, error) {
	return runBy(readinessGit(), locate, dir, revision, policy)
}

// runBy is RunWithLocator with the git that answers the questions about the policy given by the caller.
func runBy(run gitrepo.Runner, locate tstoolchain.MiseLocator, dir, revision string, policy policyref.Ref) (projectreadiness.Result, error) {
	judgment, err := judgePolicy(run, dir, revision, policy)
	if err != nil {
		return projectreadiness.Result{}, err
	}
	return evaluate(run, locate, dir, revision, judgment)
}

// evaluate is Run for a policy already judged.
func evaluate(run gitrepo.Runner, locate tstoolchain.MiseLocator, dir, revision string, judgment policyJudgment) (projectreadiness.Result, error) {
	policyPath, policyCheck, roots := judgment.path, judgment.check, judgment.roots
	projectShape, err := checkProjectShape(run, dir, revision, roots, policyCheck.State == projectreadiness.Pass)
	if err != nil {
		return projectreadiness.Result{}, err
	}
	runtime := tstoolchain.CheckNode()
	node := tstoolchain.NodeCompatibilityMirror(runtime)
	compiler := tstoolchain.ResolveCompilerWith(locate, dir, roots)
	packageManager := pkgmanager.Check(dir, roots, policyCheck.State == projectreadiness.Pass)

	checks := projectreadiness.Checks{
		ProjectShape:   projectShape,
		Policy:         policyCheck,
		Node:           node,
		Runtime:        runtime,
		Compiler:       compiler,
		PackageManager: packageManager,
	}

	dirtyRoots, err := dirtyRelevanceRoots(run, dir, revision, judgment)
	if err != nil {
		return projectreadiness.Result{}, err
	}
	dirty, err := detectRelevantDirtyWorktree(dir, dirtyRoots, policyPath)
	if err != nil {
		return projectreadiness.Result{}, err
	}

	miseChoices := tstoolchain.EvaluateMiseSetupChoices(dir, roots)
	status, gaps, nextActions, warnings := aggregateReadiness(checks, dirty.RelevantChanges, miseChoices)
	warnings = append(warnings, judgment.warnings...)

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
