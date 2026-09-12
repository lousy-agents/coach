package codesignalcli

func aggregateReadiness(checks ReadinessChecks, dirtyRelevant bool) (ReadinessStatus, []ReadinessGap, []ReadinessNextAction, []ReadinessWarning) {
	failing := failingReadinessChecks(checks)
	gaps, nextActions, status := readinessFromGapChecks(failing)
	if len(gaps) == 0 && hasReadinessLimitWarning(checks, dirtyRelevant) {
		status = StatusReadyWithLimits
	}
	return status, gaps, nextActions, readinessWarnings(checks)
}

// failingReadinessChecks reads checks.Runtime rather than checks.Node: the
// two always carry the same State/Code (see nodeCompatibilityMirror), and
// including both here would double-report every Node gap.
//
// checks.PackageManager is excluded once checks.Compiler already passes
// (SA-280-045): once a supported compiler resolves, a package-manager
// finding stays visible on the check itself but contributes no gap, next
// action, or status change.
func failingReadinessChecks(checks ReadinessChecks) []ReadinessCheck {
	var failing []ReadinessCheck
	for _, check := range []ReadinessCheck{checks.ProjectShape, checks.Policy, checks.Runtime, checks.Compiler} {
		if check.State == ReadinessFail {
			failing = append(failing, check)
		}
	}
	if checks.PackageManager.State == ReadinessFail && checks.Compiler.State != ReadinessPass {
		failing = append(failing, checks.PackageManager)
	}
	return failing
}

func readinessFromGapChecks(failing []ReadinessCheck) ([]ReadinessGap, []ReadinessNextAction, ReadinessStatus) {
	gaps := make([]ReadinessGap, 0, len(failing))
	nextActions := make([]ReadinessNextAction, 0, len(failing))
	seenActions := map[string]bool{}
	status := StatusReady
	for _, check := range failing {
		gaps = append(gaps, ReadinessGap{Code: check.Code})
		if candidate := statusForGapCode(check.Code); statusRank(candidate) > statusRank(status) {
			status = candidate
		}
		if kind, ok := nextActionForGapCode(check.Code); ok && !seenActions[kind] {
			seenActions[kind] = true
			nextActions = append(nextActions, nextActionForCheck(kind, check))
		}
	}
	return gaps, nextActions, status
}

// nextActionForCheck fills each kind's frozen payload variant (SA-280-032):
// no field appears on a kind that does not list it in the parent epic's
// gap-to-action table.
func nextActionForCheck(kind string, check ReadinessCheck) ReadinessNextAction {
	action := ReadinessNextAction{Kind: kind, Executable: nextActionExecutable(kind)}
	switch kind {
	case nextActionKindInstallSupportedRuntime:
		action.RuntimeKind = readinessNodeCheckKind
		action.Supported = supportedNodeMajorsCopy()
		if check.Code == GapNodeUnsupported {
			action.FoundVersion = check.Version
		}
	case nextActionKindRepairRuntimeProbe:
		action.RuntimeKind = readinessNodeCheckKind
		action.Detail = check.Detail
	case nextActionKindPrepareCompiler:
		action.Supported = supportedTypescriptVersionsCopy()
		action.FoundVersion = check.FoundVersion
	case nextActionKindResolvePackageManager:
		action.PackageManagerKind = check.Kind
		action.FoundVersion = check.FoundVersion
	}
	return action
}

func hasReadinessLimitWarning(checks ReadinessChecks, dirtyRelevant bool) bool {
	return dirtyRelevant || checks.Compiler.Code == WarnCompilerDeclarationMismatch
}

func readinessWarnings(checks ReadinessChecks) []ReadinessWarning {
	warnings := make([]ReadinessWarning, 0, 1)
	warnings = append(warnings, compilerDeclarationWarnings(checks.Compiler)...)
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
