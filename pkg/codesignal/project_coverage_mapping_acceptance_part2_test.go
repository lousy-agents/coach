package codesignal_test

import (
	"github.com/lousy-agents/coach/pkg/domain"
)

// baseProjectScope returns a distinct ProjectScope for the base revision in diff-mode tests.
// Its candidate_files (7) and layers differ from headProjectScope's so base/head conflation
// is detectable without asserting pointer equality.
func baseProjectScope() *domain.ProjectScope {
	return &domain.ProjectScope{
		InclusionRule:   domain.InclusionRuleTSConfigIncludesNoTestClassification,
		PatternSet:      domain.TSReachabilityAlgorithm,
		Roots:           []domain.ProjectScopeRoot{{Root: ".", CandidateFiles: 7, AnalyzedFiles: 5}},
		MatchedLayers:   []string{"api"},
		UnmatchedLayers: []string{"cache", "db"},
	}
}
