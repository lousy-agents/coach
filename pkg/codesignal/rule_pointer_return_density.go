package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const defaultPointerReturnDensityRecommendation = "Reconsider whether all of these functions need to return pointers; returning values where mutation and nil-ness aren't required can simplify ownership and reduce aliasing risk."

const pointerReturnDensityWhyItMatters = "A high concentration of pointer-returning functions in one file can indicate widespread reliance on aliasing and shared mutable state, which makes ownership harder to reason about and increases the risk of unintended mutation through shared references."

// newPointerReturnDensitySignal builds a signal for one pointer_return
// finding. The caller is responsible for applying the density gate (only
// call this once the file's pointer_return count reaches the threshold);
// this constructor does not re-check the count.
func newPointerReturnDensitySignal(path string, finding semantics.Finding) Signal {
	recommendation := finding.Recommendation
	if recommendation == "" {
		recommendation = defaultPointerReturnDensityRecommendation
	}

	return Signal{
		RuleID:         "structure.pointer_return_density",
		RuleVersion:    "1",
		Kind:           "pointer_return_density",
		Category:       "structure",
		Severity:       "low",
		Confidence:     "medium",
		Path:           path,
		Subject:        finding.Name,
		Location:       finding.Location,
		Evidence:       finding.Evidence,
		WhyItMatters:   pointerReturnDensityWhyItMatters,
		Recommendation: recommendation,
		SuggestedSkill: finding.SuggestedSkill,
		Provenance: Provenance{
			Producer:    "semantics",
			FindingKind: "pointer_return",
		},
	}
}
