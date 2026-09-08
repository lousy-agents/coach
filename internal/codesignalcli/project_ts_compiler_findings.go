package codesignalcli

import "strings"

func (a compilerAggregate) firstOfClass(class string) (compilerCandidate, bool) {
	for _, candidate := range a.candidates {
		if candidate.class == class {
			return candidate, true
		}
	}
	return compilerCandidate{}, false
}

// declarationMismatches lists the selected roots whose manifest disagrees
// with the winning compiler. A winning non-project origin disagrees with any
// differing declaration; when the project origin itself wins, only a stale
// exact pin disagrees -- range satisfaction is never evaluated (epic #280,
// owner decision D4).
func (a compilerAggregate) declarationMismatches() []ReadinessDeclarationMismatch {
	if a.winner == nil {
		return nil
	}
	projectWon := a.winner.origin == compilerOriginProject
	var mismatches []ReadinessDeclarationMismatch
	for _, declaration := range a.project.declarations {
		if declaration.declared == a.winner.version {
			continue
		}
		if projectWon && !isExactVersion(declaration.declared) {
			continue
		}
		mismatches = append(mismatches, ReadinessDeclarationMismatch{Root: declaration.root, Declared: declaration.declared})
	}
	return mismatches
}

func (a compilerAggregate) namedDeclaration() string {
	if a.project.rejectedDeclaration != "" {
		return a.project.rejectedDeclaration
	}
	if len(a.project.declarations) > 0 {
		return a.project.declarations[0].declared
	}
	return ""
}

func (a compilerAggregate) originFindings() []ReadinessOriginFinding {
	findings := make([]ReadinessOriginFinding, 0, len(a.candidates))
	for _, candidate := range a.candidates {
		findings = append(findings, ReadinessOriginFinding{Origin: candidate.origin, Class: candidate.class})
	}
	return findings
}

func formatOriginFindings(findings []ReadinessOriginFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, finding.Origin+":"+finding.Class)
	}
	return strings.Join(parts, ",")
}
