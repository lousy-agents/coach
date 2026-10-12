package main

func declarationMismatchWarning(doc readinessResultDoc) (declared, found, origin string, present bool) {
	for _, warning := range doc.Warnings {
		if warning.Code == "compiler_declaration_mismatch" {
			return warning.DeclaredVersion, warning.FoundVersion, warning.DeclarationOrigin, true
		}
	}
	return "", "", "", false
}

func rootFindingPairs(doc readinessResultDoc) []string {
	pairs := make([]string, 0, len(doc.Checks.Compiler.RootFindings))
	for _, finding := range doc.Checks.Compiler.RootFindings {
		if finding.Version == "" {
			pairs = append(pairs, finding.Root)
			continue
		}
		pairs = append(pairs, finding.Root+"@"+finding.Version)
	}
	return pairs
}

func prepareCompilerChoices(doc readinessResultDoc) ([]string, bool) {
	for _, action := range doc.NextActions {
		if action.Kind == "prepare_compiler" {
			return action.Choices, true
		}
	}
	return nil, false
}

func warningCodes(doc readinessResultDoc) []string {
	codes := make([]string, 0, len(doc.Warnings))
	for _, warning := range doc.Warnings {
		codes = append(codes, warning.Code)
	}
	return codes
}
