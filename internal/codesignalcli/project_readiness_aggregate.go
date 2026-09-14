package codesignalcli

// ReadinessMiseChoice is one mise-origin setup-choice input to
// aggregateReadiness. Kind distinguishes "mise_project" from "mise_global"
// so the two configuration scopes stay independently verifiable and
// withholdable. Verified means mise itself resolved a supported,
// hazard-free compiler-install origin at Kind; when Verified is false, Code
// names the package_manager_* gap that rejects this specific choice,
// without touching any other choice. Reason is what AvailableSetupChoices
// tells a customer when it withholds this scope from the setup menu, and is
// deliberately not Code: a trusted scope that simply pins nothing installable
// is withheld with no gap code at all, which Code alone cannot express.
type ReadinessMiseChoice struct {
	Kind     string
	Verified bool
	Code     string
	Reason   string
}

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

// packageManagerGapEntries: the four package_manager_* codes are
// setup-scoped, reported only while checks.Compiler has not passed, and
// rejecting one installation choice (the project adapter, or a specific
// mise origin) never withholds a different, still-verified choice. It
// returns the package-manager gaps/next-actions to append, their worst
// status contribution, and the Kind of every choice verified as usable for
// compiler preparation.
func packageManagerGapEntries(checks ReadinessChecks, miseChoices []ReadinessMiseChoice) (gaps []ReadinessGap, actions []ReadinessNextAction, status ReadinessStatus, verified []string) {
	status = StatusReady
	if checks.Compiler.State == ReadinessPass {
		return nil, nil, status, nil
	}

	if checks.PackageManager.State == ReadinessPass && checks.PackageManager.Kind != "" {
		verified = append(verified, checks.PackageManager.Kind)
	}
	if checks.PackageManager.State == ReadinessFail && isPackageManagerGapCode(checks.PackageManager.Code) {
		gaps, actions, status = appendPackageManagerFinding(gaps, actions, status, checks.PackageManager.Code, checks.PackageManager.Kind, checks.PackageManager.FoundVersion)
	}

	for _, choice := range miseChoices {
		if choice.Verified {
			verified = append(verified, choice.Kind)
			continue
		}
		if !isPackageManagerGapCode(choice.Code) {
			continue
		}
		gaps, actions, status = appendPackageManagerFinding(gaps, actions, status, choice.Code, choice.Kind, "")
	}

	return gaps, actions, status, verified
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

// restrictPrepareCompilerChoices: once the project package-manager adapter
// has actually been evaluated and rejected (checks.PackageManager.State ==
// ReadinessFail, passed as adapterRejected), a rejected installation choice
// never appears in prepare_compiler's Choices, and prepare_compiler is
// withheld entirely when no verified installation choice remains. An
// adapter that has not been evaluated (ReadinessNotChecked -- no recognized
// packageManager field or lockfile at all) must never by itself count as
// "no installation choice exists" -- only a genuine rejection does, so this
// is a no-op unless adapterRejected is true.
func restrictPrepareCompilerChoices(actions []ReadinessNextAction, adapterRejected bool, verified []string) []ReadinessNextAction {
	if !adapterRejected {
		return actions
	}
	restricted := make([]ReadinessNextAction, 0, len(actions))
	for _, action := range actions {
		if action.Kind != nextActionKindPrepareCompiler {
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
