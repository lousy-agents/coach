package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const branchDensityWhyItMatters = "A large number of branching constructs concentrated in one file increases the number of paths a reader has to hold in mind at once and the number of cases tests need to cover."

const branchDensityRecommendation = "Split this file's branching logic across smaller, single-purpose functions or files so each piece has fewer paths to reason about."

// branchSum totals metrics fields that represent a branch point. TypeSwitches
// and Selects are Go-only and stay 0 for TS/TSX, so the same formula applies
// to every language.
func branchSum(metrics semantics.StructuralMetrics) int {
	return metrics.Ifs + metrics.Fors + metrics.ExprSwitches + metrics.TypeSwitches + metrics.Selects
}

// newBranchDensitySignal builds a complexity.branch_density signal from
// metrics when branchSum(metrics) reaches the branchDensityRule threshold, or
// reports ok=false otherwise.
func newBranchDensitySignal(path string, metrics semantics.StructuralMetrics) (signal Signal, ok bool) {
	sum := branchSum(metrics)
	if !branchDensityRule.reaches(sum) {
		return Signal{}, false
	}

	return Signal{
		RuleID:         branchDensityRule.ruleID,
		RuleVersion:    "1",
		Kind:           "branch_density",
		Category:       "complexity",
		Severity:       branchDensityRule.severity(sum),
		Confidence:     "medium",
		Path:           path,
		Evidence:       branchDensityRule.evidence(sum),
		WhyItMatters:   branchDensityWhyItMatters,
		Recommendation: branchDensityRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}, true
}
