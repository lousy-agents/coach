package codesignalcli

import "strings"

func declaredTypescriptVersion(manifestFields map[string]string) (declaration string, conflict bool) {
	var exactDeclared []string
	var nonExactDeclared []string
	for _, field := range []string{"dependencies", "devDependencies"} {
		value, ok := manifestFields[field]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if isExactVersion(value) {
			exactDeclared = append(exactDeclared, value)
			continue
		}
		nonExactDeclared = append(nonExactDeclared, value)
	}
	uniqueExact := dedupeStrings(exactDeclared)
	uniqueNonExact := dedupeStrings(nonExactDeclared)
	if len(uniqueExact) > 1 || len(uniqueNonExact) > 1 {
		return "", true
	}
	if len(uniqueExact) == 1 {
		return uniqueExact[0], false
	}
	if len(uniqueNonExact) == 1 {
		return uniqueNonExact[0], false
	}
	return "", false
}

func disqualifyingDeclaration(declaration string) bool {
	if declaration == "" {
		return false
	}
	return !isExactVersion(declaration) || !isSupportedTypescriptVersion(declaration)
}
