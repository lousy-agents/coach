package codesignalcli

import (
	"errors"
	"strconv"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var SupportedNodeMajors = []int{24, 26}

func nodeMajorSupported(major int) bool {
	for _, supported := range SupportedNodeMajors {
		if supported == major {
			return true
		}
	}
	return false
}

func supportedNodeMajorsCopy() []string {
	out := make([]string, len(SupportedNodeMajors))
	for i, major := range SupportedNodeMajors {
		out[i] = strconv.Itoa(major)
	}
	return out
}

func CheckProjectReadiness(dir, revision, configPath string) (*projectreadiness.Result, error) {
	policyPath := configPath
	if policyPath == "" {
		policyPath = projectconfig.DefaultPath
	}

	policy, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return nil, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policy.State == projectreadiness.Pass)
	if err != nil {
		return nil, err
	}
	runtime := checkNodeReadiness()
	node := nodeCompatibilityMirror(runtime)
	compiler := resolveCompiler(dir, roots)
	packageManager := checkPackageManager(dir, roots, policy.State == projectreadiness.Pass)

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

	miseChoices := evaluateMiseSetupChoices(dir, roots)
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

func checkPolicy(dir, revision, policyPath string) (projectreadiness.Check, []string, error) {
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
