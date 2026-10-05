package codesignalcli

// failingReadinessChecks reads checks.Runtime rather than checks.Node: the
// two always carry the same State/Code (see nodeCompatibilityMirror), and
// including both here would double-report every Node gap. checks.PackageManager
// is deliberately excluded here -- its package_manager_* finding is
// setup-scoped (SA-280-045) and handled separately by packageManagerGapEntries,
// not by this table-driven path; including it here as well would double-report
// it.
func failingReadinessChecks(checks ReadinessChecks) []ReadinessCheck {
	var failing []ReadinessCheck
	for _, check := range []ReadinessCheck{checks.ProjectShape, checks.Policy, checks.Runtime, checks.Compiler} {
		if check.State == ReadinessFail {
			failing = append(failing, check)
		}
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
		status = raiseStatus(status, statusForGapCode(check.Code))
		if kind, ok := nextActionForGapCode(check.Code); ok && !seenActions[kind] {
			seenActions[kind] = true
			nextActions = append(nextActions, nextActionForCheck(kind, check))
		}
	}
	return gaps, nextActions, status
}

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

func raiseStatus(status, candidate ReadinessStatus) ReadinessStatus {
	if statusRank(candidate) > statusRank(status) {
		return candidate
	}
	return status
}

// appendPackageManagerFinding's foundVersion is set only for the project
// adapter's own finding: ReadinessMiseChoice carries no version, since a
// rejected mise scope is unverifiable before any version is ever read.
func appendPackageManagerFinding(gaps []ReadinessGap, actions []ReadinessNextAction, status ReadinessStatus, code, kind, foundVersion string) ([]ReadinessGap, []ReadinessNextAction, ReadinessStatus) {
	gaps = append(gaps, ReadinessGap{Code: code, PackageManagerKind: kind})
	if actionKind, ok := nextActionForGapCode(code); ok {
		actions = append(actions, ReadinessNextAction{Kind: actionKind, Executable: nextActionExecutable(actionKind), PackageManagerKind: kind, FoundVersion: foundVersion})
	}
	status = raiseStatus(status, statusForGapCode(code))
	return gaps, actions, status
}
