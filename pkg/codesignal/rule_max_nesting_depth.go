package codesignal

import (
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// maxNestingDepthThreshold is the minimum StructuralMetrics.MaxNestingDepth
// value that triggers a complexity.max_nesting_depth signal.
const maxNestingDepthThreshold = 4

const maxNestingDepthWhyItMatters = "Deeply nested control flow is harder to read, harder to test exhaustively, and hides the function's actual branching structure behind indentation, making it easy to miss an edge case."

const maxNestingDepthRecommendation = "Extract deeply nested blocks into named helper functions or invert conditionals with early returns to flatten the control flow."

// newMaxNestingDepthSignal builds a complexity.max_nesting_depth signal from
// metrics when MaxNestingDepth reaches maxNestingDepthThreshold, or reports
// ok=false otherwise.
func newMaxNestingDepthSignal(path string, metrics semantics.StructuralMetrics) (signal Signal, ok bool) {
	if metrics.MaxNestingDepth < maxNestingDepthThreshold {
		return Signal{}, false
	}

	return Signal{
		RuleID:         "complexity.max_nesting_depth",
		RuleVersion:    "1",
		Kind:           "max_nesting_depth",
		Category:       "complexity",
		Severity:       "medium",
		Confidence:     "medium",
		Path:           path,
		Evidence:       "max_nesting_depth=" + strconv.Itoa(metrics.MaxNestingDepth),
		WhyItMatters:   maxNestingDepthWhyItMatters,
		Recommendation: maxNestingDepthRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}, true
}
