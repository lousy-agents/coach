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
	policyPath, extraWarnings, err := resolveCheckPolicy(dir, revision, configPath)
	if err != nil {
		return nil, err
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
	warnings = append(warnings, extraWarnings...)

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

// ResolvePolicyPath is the policy document Run checks. An empty configPath
// is the path Discover and policyFromSelection select.
func ResolvePolicyPath(dir, revision, configPath string) (string, error) {
	path, _, err := resolveCheckPolicy(dir, revision, configPath)
	return path, err
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

func resolveCheckPolicy(dir, revision, configPath string) (string, []projectreadiness.Warning, error) {
	if configPath != "" {
		return configPath, nil, nil
	}
	selection, err := projectconfig.Discover(dir, revision)
	if err != nil {
		return "", nil, err
	}
	return policyFromSelection(selection)
}

func policyFromSelection(selection projectconfig.Selection) (string, []projectreadiness.Warning, error) {
	switch {
	case selection.Fatal != nil && !canonicalRecheckedByReadiness(selection.Fatal):
		return "", nil, selection.Fatal
	case selection.Deprecated:
		return selection.Path, []projectreadiness.Warning{{Code: "project_config_deprecated"}}, nil
	case selection.Path != "":
		return selection.Path, nil, nil
	default:
		return projectconfig.CanonicalPath, nil, nil
	}
}

// canonicalRecheckedByReadiness reports a canonical Load failure that
// CheckPolicy must classify again. Load collapses a stdout size bound and a
// timeout or stderr bound into KindUnreadable. LoadForReadiness, which
// CheckPolicy calls, reports the size bound as policy_invalid and leaves the
// timeout and stderr bounds as operational errors. Scan keeps Load's
// classification and does not call this path.
func canonicalRecheckedByReadiness(err error) bool {
	var configErr *projectconfig.ConfigError
	if !errors.As(err, &configErr) {
		return false
	}
	switch configErr.Kind {
	case projectconfig.KindInvalid, projectconfig.KindUnreadable:
		return true
	default:
		return false
	}
}
