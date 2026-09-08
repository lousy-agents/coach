package codesignalcli

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
	warnings = append(warnings, compilerDeclarationWarnings(checks.Compiler)...)
	if warning, ok := nodeUntestedWarning(checks.Node); ok {
		warnings = append(warnings, warning)
	}
	return warnings
}

func compilerDeclarationWarnings(check ReadinessCheck) []ReadinessWarning {
	if check.Code != WarnCompilerDeclarationMismatch {
		return nil
	}
	warnings := make([]ReadinessWarning, 0, len(check.DeclarationMismatches))
	for _, mismatch := range check.DeclarationMismatches {
		warnings = append(warnings, ReadinessWarning{
			Code:              WarnCompilerDeclarationMismatch,
			DeclaredVersion:   mismatch.Declared,
			FoundVersion:      check.Version,
			DeclarationOrigin: check.DeclarationOrigin,
			Root:              mismatch.Root,
		})
	}
	return warnings
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
