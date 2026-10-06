package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const maxNestingDepthWhyItMatters = "Deeply nested control flow is harder to read, harder to test exhaustively, and hides the function's actual branching structure behind indentation, making it easy to miss an edge case."

const maxNestingDepthRecommendation = "Extract deeply nested blocks into named helper functions or invert conditionals with early returns to flatten the control flow."

func newMaxNestingDepthSignal(path string, metrics semantics.StructuralMetrics) (signal Signal, ok bool) {
	if !maxNestingDepthRule.reaches(metrics.MaxNestingDepth) {
		return Signal{}, false
	}

	return Signal{
		RuleID:         maxNestingDepthRule.ruleID,
		RuleVersion:    "1",
		Kind:           "max_nesting_depth",
		Category:       "complexity",
		Severity:       maxNestingDepthRule.severity(metrics.MaxNestingDepth),
		Confidence:     "medium",
		Path:           path,
		Evidence:       maxNestingDepthRule.evidence(metrics.MaxNestingDepth),
		WhyItMatters:   maxNestingDepthWhyItMatters,
		Recommendation: maxNestingDepthRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}, true
}
