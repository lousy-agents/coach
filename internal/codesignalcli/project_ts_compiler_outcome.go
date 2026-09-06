package codesignalcli

func mismatchCompilerCheck(found string) ReadinessCheck {
	return ReadinessCheck{
		State:             ReadinessFail,
		Code:              GapTypescriptVersionMismatch,
		ExpectedVersion:   newestSupportedTypescriptVersion(),
		FoundVersion:      found,
		SupportedVersions: supportedTypescriptVersionsCopy(),
	}
}

func missingCompilerCheck(found, declared string) ReadinessCheck {
	check := ReadinessCheck{
		State:           ReadinessFail,
		Code:            GapTypescriptCompilerMissing,
		ExpectedVersion: newestSupportedTypescriptVersion(),
		DeclaredVersion: declared,
	}
	if found != "" {
		check.FoundVersion = found
	}
	return check
}

func firstPendingDecl(pending string, outcome compilerOriginOutcome) string {
	if pending == "" && outcome.warnDecl {
		return outcome.declared
	}
	return pending
}

func firstNonEmpty(current, next string) string {
	if current != "" {
		return current
	}
	return next
}

func compilerCheckFromNonEmptyOrigin(outcome compilerOriginOutcome, pendingDecl, pendingFound string) ReadinessCheck {
	switch outcome.state {
	case compilerOutcomePass:
		return passOrMissingCompilerCheck(outcome, pendingDecl, pendingFound)
	case compilerOutcomeMismatch:
		found := outcome.found
		if found == "" {
			found = outcome.version
		}
		return mismatchCompilerCheck(found)
	case compilerOutcomeConflict:
		return ReadinessCheck{State: ReadinessFail, Code: GapTypescriptVersionConflict, RootFindings: outcome.rootFindings}
	default:
		return missingCompilerCheck(pendingFound, pendingDecl)
	}
}

func passOrMissingCompilerCheck(outcome compilerOriginOutcome, pendingDecl, pendingFound string) ReadinessCheck {
	_, installed, gap := resolvedInstalledCompiler(outcome)
	switch gap {
	case GapTypescriptCompilerMissing:
		declared := firstNonEmpty(pendingDecl, outcome.version)
		return missingCompilerCheck(pendingFound, declared)
	case GapTypescriptVersionMismatch:
		return mismatchCompilerCheck(installed)
	default:
		return passingCompilerCheck(installed, pendingDecl)
	}
}

func installedCompilerVersion(location, declared string) string {
	version, exists, unreadable := readTypescriptVersionAt(location)
	if !unreadable && exists {
		return version
	}
	return declared
}

func passingCompilerCheck(installed, pendingDecl string) ReadinessCheck {
	check := ReadinessCheck{State: ReadinessPass, Version: installed}
	if pendingDecl != "" {
		check.Code = WarnCompilerDeclarationMismatch
		check.DeclaredVersion = pendingDecl
		check.DeclarationOrigin = compilerDeclarationOriginManifest
	}
	return check
}

func runtimeResolutionFromOrigin(outcome compilerOriginOutcome) (compilerRuntimeResolution, error) {
	switch outcome.state {
	case compilerOutcomePass:
		return locatableRuntimeResolution(outcome)
	case compilerOutcomeMismatch:
		return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptVersionMismatch)
	case compilerOutcomeConflict:
		return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptVersionConflict)
	default:
		return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptCompilerMissing)
	}
}

func locatableRuntimeResolution(outcome compilerOriginOutcome) (compilerRuntimeResolution, error) {
	location, installed, gap := resolvedInstalledCompiler(outcome)
	if gap != "" {
		return compilerRuntimeResolution{}, compilerUnresolved(gap)
	}
	return compilerRuntimeResolution{Origin: outcome.origin, Version: installed, Path: location}, nil
}

// resolvedInstalledCompiler locates the origin's on-disk compiler and
// returns its installed version. A non-empty gap is
// GapTypescriptCompilerMissing or GapTypescriptVersionMismatch and is the
// same code readiness and runtime resolution must report for that origin,
// so the two projections cannot drift.
func resolvedInstalledCompiler(outcome compilerOriginOutcome) (location, version, gap string) {
	location, ok := locateCompilerForOrigin(outcome)
	if !ok {
		return "", "", GapTypescriptCompilerMissing
	}
	installed := installedCompilerVersion(location, outcome.version)
	if !isSupportedTypescriptVersion(installed) {
		return location, installed, GapTypescriptVersionMismatch
	}
	return location, installed, ""
}
