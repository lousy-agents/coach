package codesignalcli

import (
	"context"
)

// miseSetupChoice withholds a trusted scope that has no installable pin
// without inventing a package_manager_* gap: missing configuration is not a
// hazard, it is simply not an executable setup choice. Trust failures still
// surface their own gap code so an untrusted or unverifiable mise is never
// silent. Reason is the finer distinction Code cannot carry, since the
// no-pin case has no gap code at all: it is what AvailableSetupChoices names
// when it withholds this scope from the menu.
func miseSetupChoice(kind, worktreeRoot string, trust miseSetupTrust) ReadinessMiseChoice {
	if !trust.trusted {
		choice := miseReadinessChoice(kind, trust)
		choice.Reason = setupChoiceReasonMiseUnverifiable
		return choice
	}
	if reason := miseScopeSetupWithholdReason(kind, worktreeRoot); reason != "" {
		return ReadinessMiseChoice{Kind: kind, Reason: reason}
	}
	return miseReadinessChoice(kind, trust)
}

// miseScopeSetupWithholdReason reports why a trusted mise scope cannot be
// offered as an installation choice, or "" when it already declares exactly
// one exact supported-set TypeScript version. A scope whose config exists but
// cannot be read is unverifiable rather than unconfigured: Coach has no basis
// to say what it declares, which is a different thing from knowing it
// declares nothing.
func miseScopeSetupWithholdReason(origin, worktreeRoot string) string {
	if origin == compilerOriginMiseProject && !miseProjectConfigReadable(worktreeRoot) && miseProjectConfigExists(worktreeRoot) {
		return setupChoiceReasonMiseUnverifiable
	}
	if _, ok := miseScopeDeclaresInstallableCompiler(origin, worktreeRoot); !ok {
		return setupChoiceReasonMiseUnconfigured
	}
	return ""
}
func evaluateCompilerOrigins(dir string, roots []string) compilerAggregate {
	worktreeRoot := compilerWorktreeRoot(dir)
	project, rootContext := evaluateProjectOrigin(worktreeRoot, roots)

	aggregate := compilerAggregate{project: rootContext}
	if aggregate.tryOrigin(project) {
		return aggregate
	}
	if aggregate.tryOrigin(evaluateMiseProjectOriginGated(worktreeRoot)) {
		return aggregate
	}
	aggregate.tryOrigin(gatedMiseOriginEvaluation(compilerOriginMiseGlobal, evaluateMiseGlobalTrust(context.Background()), evaluateMiseGlobalOrigin))
	return aggregate
}
func localMiseProjectVersionsConflict(worktreeRoot string) bool {
	data, ok := readMiseProjectConfigFile(worktreeRoot)
	if !ok {
		return false
	}
	versions := dedupeStrings(filterExactVersions(parseMiseToolsTypescriptVersions(data)))
	return len(versions) > 1
}
func conflictFindings(origin, selectedRoots []ReadinessRootFinding) []ReadinessRootFinding {
	if len(origin) > 0 {
		return origin
	}
	return selectedRoots
}
func projectMiseConfigHazard(worktreeRoot string) bool {
	data, ok := readMiseProjectConfigFile(worktreeRoot)
	if !ok {
		return false
	}
	return hasMiseConfigHazard(data)
}

// evaluateMiseSetupChoices computes the two mise ReadinessMiseChoice entries
// CheckProjectReadiness feeds to aggregateReadiness: whether each of the
// project and global mise scopes may be offered as a prepare_compiler
// installation choice. A scope is offered only when it is trusted and
// already declares exactly one exact supported-set TypeScript version:
// the frozen row is `mise install` then `mise where`, never `mise use`, so
// an install cannot create the pin origin evaluation reads. Offering a
// pin-less scope would let the user consent to an install that leaves
// checks.compiler failing.
//
// It never probes mise when the project (manifest) origin itself disagrees
// across selected roots (evaluateProjectOrigin's own conflict):
// evaluateCompilerOrigins' frozen contract is that such a disagreement never
// evaluates mise at all, so a second, independent computation of setup
// choices must honor the same rule rather than quietly reaching mise anyway.
func evaluateMiseSetupChoices(dir string, roots []string) []ReadinessMiseChoice {
	worktreeRoot := compilerWorktreeRoot(dir)
	project, _ := evaluateProjectOrigin(worktreeRoot, roots)
	if project.conflict {
		return nil
	}
	ctx := context.Background()
	return []ReadinessMiseChoice{
		miseSetupChoice(compilerOriginMiseProject, worktreeRoot, evaluateMiseProjectTrust(ctx, worktreeRoot)),
		miseSetupChoice(compilerOriginMiseGlobal, worktreeRoot, evaluateMiseGlobalTrust(ctx)),
	}
}
