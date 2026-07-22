package codesignal

import (
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// branchDensityThreshold is the minimum branch sum (see branchSum) that
// triggers a complexity.branch_density signal.
const branchDensityThreshold = 12

const branchDensityWhyItMatters = "A large number of branching constructs concentrated in one file increases the number of paths a reader has to hold in mind at once and the number of cases tests need to cover."

const branchDensityRecommendation = "Split this file's branching logic across smaller, single-purpose functions or files so each piece has fewer paths to reason about."

// branchSum totals metrics fields that represent a branch point. TypeSwitches
// and Selects are Go-only and stay 0 for TS/TSX, so the same formula applies
// to every language.
func branchSum(metrics semantics.StructuralMetrics) int {
	return metrics.Ifs + metrics.Fors + metrics.ExprSwitches + metrics.TypeSwitches + metrics.Selects
}

// newBranchDensitySignal builds a complexity.branch_density signal from
// metrics when branchSum(metrics) reaches branchDensityThreshold, or reports
// ok=false otherwise.
func newBranchDensitySignal(path string, metrics semantics.StructuralMetrics) (signal Signal, ok bool) {
	sum := branchSum(metrics)
	if sum < branchDensityThreshold {
		return Signal{}, false
	}

	return Signal{
		RuleID:         "complexity.branch_density",
		RuleVersion:    "1",
		Kind:           "branch_density",
		Category:       "complexity",
		Severity:       "medium",
		Confidence:     "medium",
		Path:           path,
		Evidence:       "branch_sum=" + strconv.Itoa(sum),
		WhyItMatters:   branchDensityWhyItMatters,
		Recommendation: branchDensityRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}, true
}
