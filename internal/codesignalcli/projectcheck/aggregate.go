package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func hasReadinessLimitWarning(checks projectreadiness.Checks, dirtyRelevant bool) bool {
	return dirtyRelevant || checks.Compiler.Code == projectreadiness.WarnCompilerDeclarationMismatch
}

func readinessWarnings(checks projectreadiness.Checks) []projectreadiness.Warning {
	warnings := make([]projectreadiness.Warning, 0, 1)
	warnings = append(warnings, compilerDeclarationWarnings(checks.Compiler)...)
	return warnings
}

// failingReadinessChecks reads checks.Runtime rather than checks.Node: the
// two always carry the same State/Code (see tstoolchain.NodeCompatibilityMirror), and
// including both here would double-report every Node gap. checks.PackageManager
// is deliberately excluded here -- its package_manager_* finding is
// setup-scoped (SA-280-045) and handled separately by packageManagerGapEntries,
// not by this table-driven path; including it here as well would double-report
// it.
func failingReadinessChecks(checks projectreadiness.Checks) []projectreadiness.Check {
	var failing []projectreadiness.Check
	for _, check := range []projectreadiness.Check{checks.ProjectShape, checks.Policy, checks.Runtime, checks.Compiler} {
		if check.State == projectreadiness.Fail {
			failing = append(failing, check)
		}
	}
	return failing
}

func readinessFromGapChecks(failing []projectreadiness.Check) ([]projectreadiness.Gap, []projectreadiness.NextAction, projectreadiness.Status) {
	gaps := make([]projectreadiness.Gap, 0, len(failing))
	nextActions := make([]projectreadiness.NextAction, 0, len(failing))
	seenActions := map[string]bool{}
	status := projectreadiness.StatusReady
	for _, check := range failing {
		gaps = append(gaps, projectreadiness.Gap{Code: check.Code})
		status = raiseStatus(status, projectreadiness.StatusForGapCode(check.Code))
		if kind, ok := projectreadiness.NextActionForGapCode(check.Code); ok && !seenActions[kind] {
			seenActions[kind] = true
			nextActions = append(nextActions, nextActionForCheck(kind, check))
		}
	}
	return gaps, nextActions, status
}

func compilerDeclarationWarnings(check projectreadiness.Check) []projectreadiness.Warning {
	if check.Code != projectreadiness.WarnCompilerDeclarationMismatch {
		return nil
	}
	warnings := make([]projectreadiness.Warning, 0, len(check.DeclarationMismatches))
	for _, mismatch := range check.DeclarationMismatches {
		warnings = append(warnings, projectreadiness.Warning{
			Code:              projectreadiness.WarnCompilerDeclarationMismatch,
			DeclaredVersion:   mismatch.Declared,
			FoundVersion:      check.Version,
			DeclarationOrigin: check.DeclarationOrigin,
			Root:              mismatch.Root,
		})
	}
	return warnings
}

func raiseStatus(status, candidate projectreadiness.Status) projectreadiness.Status {
	if projectreadiness.StatusRank(candidate) > projectreadiness.StatusRank(status) {
		return candidate
	}
	return status
}

func aggregateReadiness(checks projectreadiness.Checks, dirtyRelevant bool, miseChoices []projectreadiness.MiseChoice) (projectreadiness.Status, []projectreadiness.Gap, []projectreadiness.NextAction, []projectreadiness.Warning) {
	failing := failingReadinessChecks(checks)
	gaps, nextActions, status := readinessFromGapChecks(failing)

	pmGaps, pmActions, pmStatus, verifiedChoices := packageManagerGapEntries(checks, miseChoices)
	gaps = append(gaps, pmGaps...)
	nextActions = append(nextActions, pmActions...)
	status = raiseStatus(status, pmStatus)
	nextActions = restrictPrepareCompilerChoices(nextActions, checks.PackageManager.State == projectreadiness.Fail, verifiedChoices)

	if len(gaps) == 0 && hasReadinessLimitWarning(checks, dirtyRelevant) {
		status = projectreadiness.StatusReadyWithLimits
	}
	return status, gaps, nextActions, readinessWarnings(checks)
}
