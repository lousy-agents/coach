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

// failingReadinessChecks reads checks.Runtime rather than checks.Node: the
// two always carry the same State/Code (see nodeCompatibilityMirror), and
// including both here would double-report every Node gap. checks.PackageManager
// is deliberately excluded here -- its package_manager_* finding is
// setup-scoped (SA-280-045) and handled separately by packageManagerGapEntries,
// not by this table-driven path; including it here as well would double-report
// it.

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

// appendPackageManagerFinding's foundVersion is set only for the project
// adapter's own finding: ReadinessMiseChoice carries no version, since a
// rejected mise scope is unverifiable before any version is ever read.

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

func hasReadinessLimitWarning(checks ReadinessChecks, dirtyRelevant bool) bool {
	return dirtyRelevant || checks.Compiler.Code == WarnCompilerDeclarationMismatch
}

func readinessWarnings(checks ReadinessChecks) []ReadinessWarning {
	warnings := make([]ReadinessWarning, 0, 1)
	warnings = append(warnings, compilerDeclarationWarnings(checks.Compiler)...)
	return warnings
}
