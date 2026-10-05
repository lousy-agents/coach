package tstoolchain

import (
	"context"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// MiseScopeDeclaresInstallableCompiler reports the exact supported-set
// TypeScript version a mise scope already pins. Origin evaluation locates
// that pin after install; a missing, inexact, conflicting, or out-of-set
// declaration cannot become a passing compiler check via `mise install`
// alone.
func MiseScopeDeclaresInstallableCompiler(origin, worktreeRoot string) (version string, ok bool) {
	switch origin {
	case OriginMiseProject:
		data, readable := readMiseProjectConfigFile(worktreeRoot)
		if !readable {
			return "", false
		}
		versions := DedupeStrings(filterExactVersions(ParseMiseToolsTypescriptVersions(data)))
		if len(versions) != 1 || !IsSupportedTypescriptVersion(versions[0]) {
			return "", false
		}
		return versions[0], true
	case OriginMiseGlobal:
		version, found := DetectGlobalMiseTypescriptVersion(context.Background())
		if !found || !IsExactVersion(version) || !IsSupportedTypescriptVersion(version) {
			return "", false
		}
		return version, true
	default:
		return "", false
	}
}

func miseReadinessChoice(kind string, trust MiseTrust) projectreadiness.MiseChoice {
	return projectreadiness.MiseChoice{Kind: kind, Verified: trust.Trusted, Code: trust.Code}
}

// miseSetupChoice withholds a trusted scope that has no installable pin
// without inventing a package_manager_* gap: missing configuration is not a
// hazard, it is simply not an executable setup choice. Trust failures still
// surface their own gap code so an untrusted or unverifiable mise is never
// silent. Reason is the finer distinction Code cannot carry, since the
// no-pin case has no gap code at all: it is what AvailableSetupChoices names
// when it withholds this scope from the menu.
func miseSetupChoice(kind, worktreeRoot string, trust MiseTrust) projectreadiness.MiseChoice {
	if !trust.Trusted {
		choice := miseReadinessChoice(kind, trust)
		choice.Reason = projectreadiness.ReasonMiseUnverifiable
		return choice
	}
	if reason := miseScopeSetupWithholdReason(kind, worktreeRoot); reason != "" {
		return projectreadiness.MiseChoice{Kind: kind, Reason: reason}
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
	if origin == OriginMiseProject && !miseProjectConfigReadable(worktreeRoot) && miseProjectConfigExists(worktreeRoot) {
		return projectreadiness.ReasonMiseUnverifiable
	}
	if _, ok := MiseScopeDeclaresInstallableCompiler(origin, worktreeRoot); !ok {
		return projectreadiness.ReasonMiseUnconfigured
	}
	return ""
}

// EvaluateMiseSetupChoices computes the two mise projectreadiness.MiseChoice entries
// projectcheck.Run feeds to aggregateReadiness: whether each of the
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
// EvaluateOrigins' frozen contract is that such a disagreement never
// evaluates mise at all, so a second, independent computation of setup
// choices must honor the same rule rather than quietly reaching mise anyway.
func EvaluateMiseSetupChoices(dir string, roots []string) []projectreadiness.MiseChoice {
	worktreeRoot := WorktreeRoot(dir)
	project, _ := evaluateProjectOrigin(worktreeRoot, roots)
	if project.conflict {
		return nil
	}
	ctx := context.Background()
	return []projectreadiness.MiseChoice{
		miseSetupChoice(OriginMiseProject, worktreeRoot, EvaluateMiseProjectTrust(ctx, worktreeRoot)),
		miseSetupChoice(OriginMiseGlobal, worktreeRoot, EvaluateMiseGlobalTrust(ctx)),
	}
}
