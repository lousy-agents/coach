package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// packageManagerGapEntries: the four package_manager_* codes are
// setup-scoped, reported only while checks.Compiler has not passed, and
// rejecting one installation choice (the project adapter, or a specific
// mise origin) never withholds a different, still-verified choice. It
// returns the package-manager gaps/next-actions to append, their worst
// status contribution, and the Kind of every choice verified as usable for
// compiler preparation.
func packageManagerGapEntries(checks projectreadiness.Checks, miseChoices []projectreadiness.MiseChoice) (gaps []projectreadiness.Gap, actions []projectreadiness.NextAction, status projectreadiness.Status, verified []string) {
	status = projectreadiness.StatusReady
	if checks.Compiler.State == projectreadiness.Pass {
		return nil, nil, status, nil
	}

	if checks.PackageManager.State == projectreadiness.Pass && checks.PackageManager.Kind != "" {
		verified = append(verified, checks.PackageManager.Kind)
	}
	if checks.PackageManager.State == projectreadiness.Fail && projectreadiness.IsPackageManagerGapCode(checks.PackageManager.Code) {
		gaps, actions, status = appendPackageManagerFinding(gaps, actions, status, checks.PackageManager.Code, checks.PackageManager.Kind, checks.PackageManager.FoundVersion)
	}

	for _, choice := range miseChoices {
		if choice.Verified {
			verified = append(verified, choice.Kind)
			continue
		}
		if !projectreadiness.IsPackageManagerGapCode(choice.Code) {
			continue
		}
		gaps, actions, status = appendPackageManagerFinding(gaps, actions, status, choice.Code, choice.Kind, "")
	}

	return gaps, actions, status, verified
}

// restrictPrepareCompilerChoices: once the project package-manager adapter
// has actually been evaluated and rejected (checks.PackageManager.State ==
// projectreadiness.Fail, passed as adapterRejected), a rejected installation choice
// never appears in prepare_compiler's Choices, and prepare_compiler is
// withheld entirely when no verified installation choice remains. An
// adapter that has not been evaluated (projectreadiness.NotChecked -- no recognized
// packageManager field or lockfile at all) must never by itself count as
// "no installation choice exists" -- only a genuine rejection does, so this
// is a no-op unless adapterRejected is true.
func restrictPrepareCompilerChoices(actions []projectreadiness.NextAction, adapterRejected bool, verified []string) []projectreadiness.NextAction {
	if !adapterRejected {
		return actions
	}
	restricted := make([]projectreadiness.NextAction, 0, len(actions))
	for _, action := range actions {
		if action.Kind != projectreadiness.NextActionPrepareCompiler {
			restricted = append(restricted, action)
			continue
		}
		if len(verified) == 0 {
			continue
		}
		action.Choices = append([]string(nil), verified...)
		restricted = append(restricted, action)
	}
	return restricted
}

func hasReadinessLimitWarning(checks projectreadiness.Checks, dirtyRelevant bool) bool {
	return dirtyRelevant || checks.Compiler.Code == projectreadiness.WarnCompilerDeclarationMismatch
}

func readinessWarnings(checks projectreadiness.Checks) []projectreadiness.Warning {
	warnings := make([]projectreadiness.Warning, 0, 1)
	warnings = append(warnings, compilerDeclarationWarnings(checks.Compiler)...)
	return warnings
}
