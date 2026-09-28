package codesignalcli

func aggregateReadiness(checks ReadinessChecks, dirtyRelevant bool, miseChoices []ReadinessMiseChoice) (ReadinessStatus, []ReadinessGap, []ReadinessNextAction, []ReadinessWarning) {
	failing := failingReadinessChecks(checks)
	gaps, nextActions, status := readinessFromGapChecks(failing)

	pmGaps, pmActions, pmStatus, verifiedChoices := packageManagerGapEntries(checks, miseChoices)
	gaps = append(gaps, pmGaps...)
	nextActions = append(nextActions, pmActions...)
	status = raiseStatus(status, pmStatus)
	nextActions = restrictPrepareCompilerChoices(nextActions, checks.PackageManager.State == ReadinessFail, verifiedChoices)

	if len(gaps) == 0 && hasReadinessLimitWarning(checks, dirtyRelevant) {
		status = StatusReadyWithLimits
	}
	return status, gaps, nextActions, readinessWarnings(checks)
}
