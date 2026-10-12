package tstoolchain

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// compilerOutcomeFromAggregate is a decision table: case order is the
// precedence between conflict, a winner, an unresolved root, an unsupported
// candidate, and a missing compiler -- reordering the cases changes which
// gap code wins for an input matching more than one.
func compilerOutcomeFromAggregate(aggregate compilerAggregate) (code string, findings []projectreadiness.RootFinding) {
	switch {
	case aggregate.conflict:
		return projectreadiness.GapTypescriptVersionConflict, aggregate.findings
	case aggregate.Winner != nil:
		return "", nil
	case aggregate.project.someRootsResolvedNothing:
		return projectreadiness.GapTypescriptVersionConflict, aggregate.project.rootFindings
	}
	if unsupported, ok := aggregate.firstOfClass(ClassUnsupported); ok {
		return projectreadiness.GapTypescriptVersionMismatch, mismatchRootFindings(unsupported, aggregate.project.rootFindings)
	}
	return projectreadiness.GapTypescriptCompilerMissing, nil
}

func conflictFindings(origin, selectedRoots []projectreadiness.RootFinding) []projectreadiness.RootFinding {
	if len(origin) > 0 {
		return origin
	}
	return selectedRoots
}

func mismatchRootFindings(unsupported candidate, findings []projectreadiness.RootFinding) []projectreadiness.RootFinding {
	if unsupported.Origin != OriginProject {
		return nil
	}
	return findings
}

func compilerCheckFromAggregate(aggregate compilerAggregate) projectreadiness.Check {
	code, findings := compilerOutcomeFromAggregate(aggregate)
	switch code {
	case "":
		return passingCompilerCheck(aggregate)
	case projectreadiness.GapTypescriptVersionConflict:
		return projectreadiness.Check{State: projectreadiness.Fail, Code: code, RootFindings: findings}
	case projectreadiness.GapTypescriptVersionMismatch:
		unsupported, _ := aggregate.firstOfClass(ClassUnsupported)
		check := mismatchCompilerCheck(unsupported.Version)
		check.DeclaredVersion = aggregate.namedDeclaration()
		return check
	default:
		return missingCompilerCheckFromAggregate(aggregate)
	}
}

func passingCompilerCheck(aggregate compilerAggregate) projectreadiness.Check {
	check := projectreadiness.Check{State: projectreadiness.Pass, Version: aggregate.Winner.Version}
	if mismatches := aggregate.declarationMismatches(); len(mismatches) > 0 {
		check.Code = projectreadiness.WarnCompilerDeclarationMismatch
		check.DeclarationOrigin = DeclarationOriginManifest
		check.DeclarationMismatches = mismatches
	}
	return check
}

func mismatchCompilerCheck(found string) projectreadiness.Check {
	return projectreadiness.Check{
		State:             projectreadiness.Fail,
		Code:              projectreadiness.GapTypescriptVersionMismatch,
		ExpectedVersion:   NewestSupportedTypescriptVersion(),
		FoundVersion:      found,
		SupportedVersions: SupportedTypescriptVersionsCopy(),
	}
}

func missingCompilerCheckFromAggregate(aggregate compilerAggregate) projectreadiness.Check {
	found := ""
	detail := ""
	if nativeInvalid, ok := aggregate.firstOfClass(ClassNativeInvalid); ok {
		found = nativeInvalid.Version
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

func missingCompilerCheck(found, declared string) projectreadiness.Check {
	check := projectreadiness.Check{
		State:           projectreadiness.Fail,
		Code:            projectreadiness.GapTypescriptCompilerMissing,
		ExpectedVersion: NewestSupportedTypescriptVersion(),
		DeclaredVersion: declared,
	}
	if found != "" {
		check.FoundVersion = found
	}
	return check
}
