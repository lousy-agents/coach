package codesignalcli

func statusForGapCode(code string) ReadinessStatus {
	switch code {
	case GapUnsupportedRepositoryShape:
		return StatusOutsideSupport
	case GapNodeMissing, GapNodeBelowMinimum, GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict, GapPackageManagerAmbiguous, GapPackageManagerConfigUnverifiable:
		return StatusNeedsPrerequisite
	case GapPolicyMissing, GapPolicyInvalid:
		return StatusNeedsPolicy
	default:
		return StatusReady
	}
}

func statusRank(status ReadinessStatus) int {
	switch status {
	case StatusOutsideSupport:
		return 4
	case StatusNeedsPrerequisite:
		return 3
	case StatusNeedsPolicy:
		return 2
	case StatusReadyWithLimits:
		return 1
	default:
		return 0
	}
}

func nextActionForGapCode(code string) (string, bool) {
	switch code {
	case GapUnsupportedRepositoryShape:
		return "confirm_repository_shape", true
	case GapNodeMissing, GapNodeBelowMinimum:
		return "install_node", true
	case GapTypescriptCompilerMissing, GapTypescriptVersionMismatch, GapTypescriptVersionConflict:
		return "prepare_compiler", true
	case GapPackageManagerAmbiguous, GapPackageManagerConfigUnverifiable:
		return "resolve_package_manager", true
	case GapPolicyMissing, GapPolicyInvalid:
		return "author_policy", true
	default:
		return "", false
	}
}

func aggregateReadiness(checks ReadinessChecks, dirtyRelevant bool) (ReadinessStatus, []ReadinessGap, []ReadinessNextAction, []ReadinessWarning) {
	codes := failingReadinessCodes(checks)
	gaps, nextActions, status := readinessFromGapCodes(codes)
	if len(gaps) == 0 && hasReadinessLimitWarning(checks, dirtyRelevant) {
		status = StatusReadyWithLimits
	}
	return status, gaps, nextActions, readinessWarnings(checks)
}

func failingReadinessCodes(checks ReadinessChecks) []string {
	var codes []string
	for _, check := range []ReadinessCheck{checks.ProjectShape, checks.Policy, checks.Node, checks.Compiler, checks.PackageManager} {
		if check.State == ReadinessFail {
			codes = append(codes, check.Code)
		}
	}
	return codes
}

func readinessFromGapCodes(codes []string) ([]ReadinessGap, []ReadinessNextAction, ReadinessStatus) {
	gaps := make([]ReadinessGap, 0, len(codes))
	nextActions := make([]ReadinessNextAction, 0, len(codes))
	seenActions := map[string]bool{}
	status := StatusReady
	for _, code := range codes {
		gaps = append(gaps, ReadinessGap{Code: code})
		if candidate := statusForGapCode(code); statusRank(candidate) > statusRank(status) {
			status = candidate
		}
		if kind, ok := nextActionForGapCode(code); ok && !seenActions[kind] {
			seenActions[kind] = true
			nextActions = append(nextActions, ReadinessNextAction{Kind: kind})
		}
	}
	return gaps, nextActions, status
}

func hasReadinessLimitWarning(checks ReadinessChecks, dirtyRelevant bool) bool {
	return dirtyRelevant || checks.Node.Code == WarnNodeUntested || checks.Compiler.Code == WarnCompilerDeclarationMismatch
}

func readinessWarnings(checks ReadinessChecks) []ReadinessWarning {
	warnings := make([]ReadinessWarning, 0, 1)
	if warning, ok := compilerDeclarationWarning(checks.Compiler); ok {
		warnings = append(warnings, warning)
	}
	if warning, ok := nodeUntestedWarning(checks.Node); ok {
		warnings = append(warnings, warning)
	}
	return warnings
}

func compilerDeclarationWarning(check ReadinessCheck) (ReadinessWarning, bool) {
	if check.Code != WarnCompilerDeclarationMismatch {
		return ReadinessWarning{}, false
	}
	return ReadinessWarning{
		Code:              WarnCompilerDeclarationMismatch,
		DeclaredVersion:   check.DeclaredVersion,
		FoundVersion:      check.Version,
		DeclarationOrigin: check.DeclarationOrigin,
	}, true
}

func nodeUntestedWarning(check ReadinessCheck) (ReadinessWarning, bool) {
	if check.Code != WarnNodeUntested {
		return ReadinessWarning{}, false
	}
	// checkNodeReadiness only ever sets WarnNodeUntested after successfully
	// parsing checks.Node.Version to compare its major against
	// TestedNodeMajor, so re-parsing it here to populate found_major cannot
	// fail in practice; the error is still checked so a future change to
	// that invariant fails closed (no warning emitted) rather than
	// panicking or fabricating a zero found_major.
	foundMajor, err := parseNodeMajor(check.Version)
	if err != nil {
		return ReadinessWarning{}, false
	}
	return ReadinessWarning{
		Code:        WarnNodeUntested,
		FoundMajor:  foundMajor,
		TestedMajor: TestedNodeMajor,
		FloorMajor:  MinimumSupportedNodeMajor,
	}, true
}
