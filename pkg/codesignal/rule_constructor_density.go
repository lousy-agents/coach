package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const defaultConstructorDensityRecommendation = "Consolidate overlapping constructor-like functions in this file, or split the file so each constructor's responsibility is clearer."

const constructorDensityWhyItMatters = "A high concentration of constructor-like functions in one file often signals overlapping responsibilities or an object graph that's grown organically rather than by design, making the file harder to navigate and its construction paths harder to reason about."

// newConstructorDensitySignal builds a signal for one constructor_func
// finding. The caller is responsible for applying the density gate (only
// call this once the file's constructor_func count reaches the threshold);
// this constructor does not re-check the count.
func newConstructorDensitySignal(path string, finding semantics.Finding) Signal {
	confidence := Confidence("medium")
	switch finding.Confidence {
	case "low", "medium", "high":
		confidence = Confidence(finding.Confidence)
	}

	recommendation := finding.Recommendation
	if recommendation == "" {
		recommendation = defaultConstructorDensityRecommendation
	}

	return Signal{
		RuleID:         "structure.constructor_density",
		RuleVersion:    "1",
		Kind:           "constructor_density",
		Category:       "structure",
		Severity:       "low",
		Confidence:     confidence,
		Path:           path,
		Subject:        finding.Name,
		Location:       finding.Location,
		Evidence:       finding.Evidence,
		WhyItMatters:   constructorDensityWhyItMatters,
		Recommendation: recommendation,
		SuggestedSkill: finding.SuggestedSkill,
		Provenance: Provenance{
			Producer:    "semantics",
			FindingKind: "constructor_func",
		},
	}
}
