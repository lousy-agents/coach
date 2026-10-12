package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

// headProjectScope returns a minimal ProjectScope for use in tests.
func headProjectScope() domain.ProjectScope {
	return domain.ProjectScope{
		InclusionRule:   domain.InclusionRuleTSConfigIncludesNoTestClassification,
		PatternSet:      domain.TSReachabilityAlgorithm,
		Roots:           []domain.ProjectScopeRoot{{Root: ".", CandidateFiles: 10, AnalyzedFiles: 10}},
		MatchedLayers:   []string{"handlers"},
		UnmatchedLayers: []string{"db"},
	}
}

// baseProjectScope returns a distinct ProjectScope for the base revision in diff-mode tests.
// Its candidate_files (7) and layers differ from headProjectScope's so base/head conflation
// is detectable without asserting pointer equality.
func baseProjectScope() domain.ProjectScope {
	return domain.ProjectScope{
		InclusionRule:   domain.InclusionRuleTSConfigIncludesNoTestClassification,
		PatternSet:      domain.TSReachabilityAlgorithm,
		Roots:           []domain.ProjectScopeRoot{{Root: ".", CandidateFiles: 7, AnalyzedFiles: 5}},
		MatchedLayers:   []string{"api"},
		UnmatchedLayers: []string{"cache", "db"},
	}
}

// completeCoverage returns a complete Coverage for a given phase name.
func completeCoverage(phase string) domain.Coverage {
	return domain.Coverage{Phase: phase, Complete: true}
}

// incompleteCoverage returns an incomplete Coverage for a given phase name.
func incompleteCoverage(phase string) domain.Coverage {
	return domain.Coverage{Phase: phase, Complete: false}
}

// notRequestedCoverage returns a Coverage with Phase="not_requested" and Complete=true,
// as the TypeScript backend produces when no required_layer is configured.
func notRequestedCoverage() domain.Coverage {
	return domain.Coverage{Phase: "not_requested", Complete: true}
}

// backendUnavailableCoverage returns a Coverage containing DiagBackendUnavailable,
// as produced when the TS sidecar cannot be reached.
func backendUnavailableCoverage() domain.Coverage {
	return domain.Coverage{
		Phase:    "model",
		Complete: false,
		Diagnostics: []domain.Diagnostic{
			{Code: domain.DiagBackendUnavailable, Message: "sidecar unavailable"},
		},
	}
}

// ref returns a pointer to a copy of v, so value fixtures can fill Input's
// optional pointer fields without the specs sharing one instance.
func ref[T any](v T) *T {
	return &v
}
