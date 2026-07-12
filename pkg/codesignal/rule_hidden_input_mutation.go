package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// defaultHiddenInputMutationRecommendation is used when the underlying
// Finding.Recommendation is empty.
const defaultHiddenInputMutationRecommendation = "Return the updated value instead of mutating caller-owned state, or make the in-place behavior explicit through the API name and documentation."

// hiddenInputMutationWhyItMatters is rule-owned, deterministic text -- it
// does not depend on the finding it was derived from.
const hiddenInputMutationWhyItMatters = "Mutating a caller-owned input can create behavior that is not visible from the function signature, make outcomes dependent on call ordering, introduce temporal coupling, make tests and local reasoning more difficult, and surprise callers that expect an input to remain unchanged."

// newHiddenInputMutationSignal maps a "mutates_input" Finding for the file
// at path to the sole v0.1 rule, "state.hidden_input_mutation".
func newHiddenInputMutationSignal(path string, finding semantics.Finding) Signal {
	confidence := Confidence("medium")
	switch finding.Confidence {
	case "low", "medium", "high":
		confidence = Confidence(finding.Confidence)
	}

	recommendation := finding.Recommendation
	if recommendation == "" {
		recommendation = defaultHiddenInputMutationRecommendation
	}

	return Signal{
		RuleID:         "state.hidden_input_mutation",
		RuleVersion:    "1",
		Kind:           "hidden_input_mutation",
		Category:       "state_management",
		Severity:       "medium",
		Confidence:     confidence,
		Path:           path,
		Subject:        finding.Name,
		Location:       finding.Location,
		Evidence:       finding.Evidence,
		WhyItMatters:   hiddenInputMutationWhyItMatters,
		Recommendation: recommendation,
		SuggestedSkill: finding.SuggestedSkill,
		Provenance: Provenance{
			Producer:    "semantics",
			FindingKind: "mutates_input",
		},
	}
}
