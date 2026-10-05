package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

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
