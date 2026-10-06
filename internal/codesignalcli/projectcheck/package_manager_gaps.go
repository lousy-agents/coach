package projectcheck

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

// appendPackageManagerFinding's foundVersion is set only for the project
// adapter's own finding: projectreadiness.MiseChoice carries no version, since a
// rejected mise scope is unverifiable before any version is ever read.
func appendPackageManagerFinding(gaps []projectreadiness.Gap, actions []projectreadiness.NextAction, status projectreadiness.Status, code, kind, foundVersion string) ([]projectreadiness.Gap, []projectreadiness.NextAction, projectreadiness.Status) {
	gaps = append(gaps, projectreadiness.Gap{Code: code, PackageManagerKind: kind})
	if actionKind, ok := projectreadiness.NextActionForGapCode(code); ok {
		actions = append(actions, projectreadiness.NextAction{Kind: actionKind, Executable: projectreadiness.NextActionExecutable(actionKind), PackageManagerKind: kind, FoundVersion: foundVersion})
	}
	status = raiseStatus(status, projectreadiness.StatusForGapCode(code))
	return gaps, actions, status
}
