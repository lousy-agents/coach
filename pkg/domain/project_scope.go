package domain

import (
	"sort"
)

// InclusionRuleTSConfigIncludesNoTestClassification is the inclusion rule
// recorded on a TypeScript ProjectScope.
const InclusionRuleTSConfigIncludesNoTestClassification = "tsconfig_includes_no_test_classification"

// ProjectScopeRoot is one policy root's candidate and analyzed file counts.
type ProjectScopeRoot struct {
	Root           string `json:"root"`
	CandidateFiles int    `json:"candidate_files"`
	AnalyzedFiles  int    `json:"analyzed_files"`
}

// ProjectScope is the per-revision scope classification: which policy roots
// were analyzed and which configured layers matched those files.
type ProjectScope struct {
	InclusionRule   string             `json:"inclusion_rule"`
	PatternSet      string             `json:"pattern_set"`
	Roots           []ProjectScopeRoot `json:"roots"`
	MatchedLayers   []string           `json:"matched_layers"`
	UnmatchedLayers []string           `json:"unmatched_layers"`
}

// RootScope is the candidate and analyzed file set for one policy root.
type RootScope struct {
	Root            string   `json:"root"`
	CandidateFiles  int      `json:"candidate_files"`
	AnalyzedFiles   int      `json:"analyzed_files"`
	AnalyzedPaths   []string `json:"analyzed_paths,omitempty"`
	UnanalyzedPaths []string `json:"unanalyzed_paths,omitempty"`
}

// canonicalRootScopes assumes Root is unique per Model. A duplicate Root is
// unsupported input with unspecified relative order.
func canonicalRootScopes(in []RootScope) []RootScope {
	if len(in) == 0 {
		return in
	}
	out := append([]RootScope(nil), in...)
	for i := range out {
		out[i].AnalyzedPaths = sortedStrings(out[i].AnalyzedPaths)
		out[i].UnanalyzedPaths = sortedStrings(out[i].UnanalyzedPaths)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Root < out[j].Root
	})
	return out
}
