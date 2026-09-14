package codesignalcli

func compilerCheckFromAggregate(aggregate compilerAggregate) ReadinessCheck {
	code, findings := compilerOutcomeFromAggregate(aggregate)
	switch code {
	case "":
		return passingCompilerCheck(aggregate)
	case GapTypescriptVersionConflict:
		return ReadinessCheck{State: ReadinessFail, Code: code, RootFindings: findings}
	case GapTypescriptVersionMismatch:
		unsupported, _ := aggregate.firstOfClass(compilerClassUnsupported)
		check := mismatchCompilerCheck(unsupported.version)
		check.DeclaredVersion = aggregate.namedDeclaration()
		return check
	default:
		return missingCompilerCheckFromAggregate(aggregate)
	}
}

func passingCompilerCheck(aggregate compilerAggregate) ReadinessCheck {
	check := ReadinessCheck{State: ReadinessPass, Version: aggregate.winner.version}
	if mismatches := aggregate.declarationMismatches(); len(mismatches) > 0 {
		check.Code = WarnCompilerDeclarationMismatch
		check.DeclarationOrigin = compilerDeclarationOriginManifest
		check.DeclarationMismatches = mismatches
	}
	return check
}

func mismatchCompilerCheck(found string) ReadinessCheck {
	return ReadinessCheck{
		State:             ReadinessFail,
		Code:              GapTypescriptVersionMismatch,
		ExpectedVersion:   newestSupportedTypescriptVersion(),
		FoundVersion:      found,
		SupportedVersions: supportedTypescriptVersionsCopy(),
	}
}

func missingCompilerCheckFromAggregate(aggregate compilerAggregate) ReadinessCheck {
	found := ""
	detail := ""
	if nativeInvalid, ok := aggregate.firstOfClass(compilerClassNativeInvalid); ok {
		found = nativeInvalid.version
		detail = NativeTypescriptPackageName()
	}

	// namedDeclaration, not rejectedDeclaration alone: appendProjectPackageChoice
	// gates on whether the manifest declares a version a frozen install could
	// realize, which cannot be answered from the disqualifying declarations
	// only -- an in-set declaration has to reach it too, or every repository
	// would look like one that declares nothing.
	check := missingCompilerCheck(found, aggregate.namedDeclaration())
	check.Detail = detail
	check.OriginFindings = aggregate.originFindings()
	return check
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
