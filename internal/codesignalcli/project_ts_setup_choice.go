package codesignalcli

// SetupChoiceKind names one selectable entry AvailableSetupChoices may offer
// to resolve a failing compiler check. The four kinds are frozen: callers
// switch on the constant, never on a compiler-resolution origin string.
type SetupChoiceKind string

const (
	SetupChoiceProjectPackage SetupChoiceKind = "project_package"
	SetupChoiceProjectMise    SetupChoiceKind = "project_mise"
	SetupChoiceGlobalMise     SetupChoiceKind = "global_mise"
	SetupChoiceCancel         SetupChoiceKind = "cancel"
)

const (
	setupChoiceReasonManifestDeclaration      = "manifest_declaration"
	setupChoiceReasonMiseUnconfigured         = "mise_unconfigured"
	setupChoiceReasonMiseUnverifiable         = "mise_unverifiable"
	setupChoiceReasonOriginUnverified         = "origin_unverified"
	setupChoiceReasonPackageManagerNotChecked = "package_manager_not_checked"
)

// SetupChoice is one entry AvailableSetupChoices offers as executable in the
// current environment.
type SetupChoice struct {
	Kind SetupChoiceKind
}

// WithheldSetupChoice names one candidate AvailableSetupChoices considered
// but did not offer, and why -- so a caller can tell a customer what was
// ruled out, not just what remains.
type WithheldSetupChoice struct {
	Kind   SetupChoiceKind
	Reason string
}

// SetupChoiceMenu is AvailableSetupChoices' result. Choices never carries a
// selected or recommended entry: an ambiguous package manager sets
// RequiresExplicitSelection instead of defaulting to one.
type SetupChoiceMenu struct {
	Choices                   []SetupChoice
	Withheld                  []WithheldSetupChoice
	RequiresExplicitSelection bool
}

// AvailableSetupChoices reports the setup choices available to resolve
// readiness's compiler check, if it is failing. It decides purely from
// fields CheckProjectReadiness already populated -- checks.package_manager's
// classification (SA-280-012), the compiler check's declared version, and
// readiness.MiseChoices -- rather than repeating any detection itself. A menu
// with no choices and no withheld entries means checks.compiler is not
// failing, so no setup is needed.
func AvailableSetupChoices(readiness ReadinessResult) SetupChoiceMenu {
	if readiness.Checks.Compiler.State != ReadinessFail {
		return SetupChoiceMenu{}
	}

	menu := SetupChoiceMenu{
		RequiresExplicitSelection: readiness.Checks.PackageManager.Code == GapPackageManagerAmbiguous,
	}
	menu = appendProjectPackageChoice(menu, readiness.Checks)
	menu = appendMiseChoice(menu, readiness.MiseChoices, compilerOriginMiseProject, SetupChoiceProjectMise)
	menu = appendMiseChoice(menu, readiness.MiseChoices, compilerOriginMiseGlobal, SetupChoiceGlobalMise)
	menu.Choices = append(menu.Choices, SetupChoice{Kind: SetupChoiceCancel})
	return menu
}

// appendProjectPackageChoice offers project-package only for the one shape
// where the frozen adapter row actually lands a supported compiler: the
// selected manifest declares exactly one exact supported-set typescript
// version, and checks.package_manager passed -- which is also what proves a
// readable lockfile is present, since every frozen row's locked install
// refuses to run without one (requireReadableLockfile). This mirrors the rule
// the mise scopes already follow (miseScopeDeclaresInstallableCompiler): a
// row that installs what is declared cannot install what is not declared, so
// a manifest declaring no typescript, a range, or an out-of-set version is
// withheld as manifest_declaration rather than offered as a choice whose
// install would leave checks.compiler failing exactly as it was.
//
// The other two withholding reasons are about the manager rather than the
// declaration: checks.package_manager rejected the repository's manager
// (SA-280-012), or never resolved one at all (ReadinessNotChecked -- no
// recognized packageManager field or lockfile at any selected root). Only
// ReadinessPass carries a Kind BuildSetupPreview can resolve to a frozen
// adapter row. DeclaredVersion is populated on both the
// typescript_compiler_missing and typescript_version_mismatch outcomes
// (compilerCheckFromAggregate), so this reads the declaration itself rather
// than the failing code. No reason here changes which compiler a scan uses if
// one is already installed (owner decision D4); all only withhold a setup
// choice.
func appendProjectPackageChoice(menu SetupChoiceMenu, checks ReadinessChecks) SetupChoiceMenu {
	if checks.PackageManager.State == ReadinessFail {
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: checks.PackageManager.Code})
		return menu
	}
	if checks.PackageManager.State != ReadinessPass {
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: setupChoiceReasonPackageManagerNotChecked})
		return menu
	}
	if !installableDeclaration(checks.Compiler.DeclaredVersion) {
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: setupChoiceReasonManifestDeclaration})
		return menu
	}
	menu.Choices = append(menu.Choices, SetupChoice{Kind: SetupChoiceProjectPackage})
	return menu
}

// installableDeclaration reports whether a frozen project-package row could
// realize declaration as a passing compiler check. Its positive form is the
// point: an absent declaration is as unusable as a disqualifying one, which
// disqualifyingDeclaration's negative form (used for the separate D4
// resolution question) deliberately does not say.
func installableDeclaration(declaration string) bool {
	return isExactVersion(declaration) && isSupportedTypescriptVersion(declaration)
}

// appendMiseChoice offers kind exactly when the readiness pipeline already
// verified that scope as an installation choice (evaluateMiseSetupChoices,
// owner decision D5). It deliberately does not read checks.compiler's origin
// classes: those answer a different question -- where a compiler already is
// -- and a scope can be absent there (nothing installed) while being
// uninstallable here (nothing installable pinned), which is precisely the
// case a class-driven menu offered and an install would not have resolved.
// Fail-closed: an origin the pipeline reported nothing about at all is
// withheld as unverifiable rather than offered as a default, which is what a
// caller sees when compiler-origin evaluation never reached mise.
func appendMiseChoice(menu SetupChoiceMenu, miseChoices []ReadinessMiseChoice, origin string, kind SetupChoiceKind) SetupChoiceMenu {
	for _, choice := range miseChoices {
		if choice.Kind != origin {
			continue
		}
		if choice.Verified {
			menu.Choices = append(menu.Choices, SetupChoice{Kind: kind})
			return menu
		}
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: kind, Reason: choice.Reason})
		return menu
	}
	menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: kind, Reason: setupChoiceReasonOriginUnverified})
	return menu
}
