package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const defaultTightConstructorInitRecommendation = "Inject collaborators through the constructor's parameters or setters instead of instantiating them directly inside it, so tests and callers can substitute alternatives."

const tightConstructorInitWhyItMatters = "A constructor that directly instantiates its own collaborators hardwires those dependencies, making the type harder to test in isolation, harder to substitute alternative implementations for, and more prone to hidden coupling that only surfaces at runtime."

func newTightCouplingSignal(path string, finding semantics.Finding) Signal {
	confidence := Confidence("medium")
	switch finding.Confidence {
	case "low", "medium", "high":
		confidence = Confidence(finding.Confidence)
	}

	recommendation := finding.Recommendation
	if recommendation == "" {
		recommendation = defaultTightConstructorInitRecommendation
	}

	return Signal{
		RuleID:         "coupling.tight_constructor_init",
		RuleVersion:    "1",
		Kind:           "tight_constructor_init",
		Category:       "coupling",
		Severity:       "medium",
		Confidence:     confidence,
		Path:           path,
		Subject:        finding.Name,
		Location:       finding.Location,
		Evidence:       finding.Evidence,
		WhyItMatters:   tightConstructorInitWhyItMatters,
		Recommendation: recommendation,
		SuggestedSkill: finding.SuggestedSkill,
		Provenance: Provenance{
			Producer:    "semantics",
			FindingKind: "tight_coupling",
		},
	}
}
