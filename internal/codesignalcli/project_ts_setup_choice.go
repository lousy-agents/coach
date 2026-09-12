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
	setupChoiceReasonManifestDeclaration = "manifest_declaration"
	setupChoiceReasonMiseUnconfigured    = "mise_unconfigured"
	setupChoiceReasonMiseUnverifiable    = "mise_unverifiable"
	setupChoiceReasonOriginUnverified    = "origin_unverified"
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
// classification (SA-280-012), the compiler check's declared-version and
// origin findings -- rather than repeating any detection itself. A menu with
// no choices and no withheld entries means checks.compiler is not failing,
// so no setup is needed.
func AvailableSetupChoices(readiness ReadinessResult) SetupChoiceMenu {
	if readiness.Checks.Compiler.State != ReadinessFail {
		return SetupChoiceMenu{}
	}

	menu := SetupChoiceMenu{
		RequiresExplicitSelection: readiness.Checks.PackageManager.Code == GapPackageManagerAmbiguous,
	}
	appendProjectPackageChoice(&menu, readiness.Checks)
	appendMiseChoice(&menu, readiness.Checks.Compiler, compilerOriginMiseProject, SetupChoiceProjectMise)
	appendMiseChoice(&menu, readiness.Checks.Compiler, compilerOriginMiseGlobal, SetupChoiceGlobalMise)
	menu.Choices = append(menu.Choices, SetupChoice{Kind: SetupChoiceCancel})
	return menu
}

// appendProjectPackageChoice withholds project-package for two independent
// reasons: checks.package_manager rejected the repository's manager
// (SA-280-012), or the selected manifest declares a disqualifying typescript
// version (AC-SET-11) -- installing through that manifest's own package
// manager would just reinstall the already-declared, disqualifying version.
// DeclaredVersion is populated on both the typescript_compiler_missing and
// typescript_version_mismatch outcomes (compilerCheckFromAggregate), so this
// checks the declaration itself rather than the failing code: an in-set
// exact declaration surfaced on the mismatch outcome is not disqualifying.
// Neither reason changes which compiler a scan uses if one is already
// installed (owner decision D4); both only withhold a setup choice.
func appendProjectPackageChoice(menu *SetupChoiceMenu, checks ReadinessChecks) {
	if checks.PackageManager.State == ReadinessFail {
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: checks.PackageManager.Code})
		return
	}
	if disqualifyingDeclaration(checks.Compiler.DeclaredVersion) {
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: SetupChoiceProjectPackage, Reason: setupChoiceReasonManifestDeclaration})
		return
	}
	menu.Choices = append(menu.Choices, SetupChoice{Kind: SetupChoiceProjectPackage})
}

// appendMiseChoice offers kind when origin appears in the compiler
// resolution's candidate set with something configured to act on.
// OriginFindings only populates on a typescript_compiler_missing outcome
// (missingCompilerCheckFromAggregate); any other failing outcome -- notably
// typescript_version_conflict and typescript_version_mismatch -- carries no
// origin findings at all, including the ambiguous-mise-config case that
// produced typescript_version_conflict in the first place. Fail-closed: with
// no evidence for or against an origin, it is withheld as unverifiable
// rather than offered as a default.
func appendMiseChoice(menu *SetupChoiceMenu, compiler ReadinessCheck, origin string, kind SetupChoiceKind) {
	if len(compiler.OriginFindings) == 0 {
		menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: kind, Reason: setupChoiceReasonOriginUnverified})
		return
	}
	for _, finding := range compiler.OriginFindings {
		if finding.Origin != origin {
			continue
		}
		switch finding.Class {
		case compilerClassUnconfigured:
			menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: kind, Reason: setupChoiceReasonMiseUnconfigured})
		case compilerClassUnreadable:
			menu.Withheld = append(menu.Withheld, WithheldSetupChoice{Kind: kind, Reason: setupChoiceReasonMiseUnverifiable})
		default:
			menu.Choices = append(menu.Choices, SetupChoice{Kind: kind})
		}
		return
	}
}
