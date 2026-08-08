package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const defaultTOCTOUCheckThenActRecommendation = "Attempt the filesystem operation directly and handle its error (try/catch or check the returned error) instead of checking existence first; a re-check between the check and the act does not close the race."

const toctouCheckThenActWhyItMatters = "Checking for a file's existence and then acting on it in a separate step leaves a non-atomic check-then-act window (CWE-367, Time-of-Check Time-of-Use) in which another process or actor can create, delete, or replace the file between the check and the act, causing the acted-upon state to differ from what was checked."

func newTOCTOUCheckThenActSignal(path string, finding semantics.Finding) Signal {
	confidence := Confidence("medium")
	switch finding.Confidence {
	case "low", "medium", "high":
		confidence = Confidence(finding.Confidence)
	}

	recommendation := finding.Recommendation
	if recommendation == "" {
		recommendation = defaultTOCTOUCheckThenActRecommendation
	}

	return Signal{
		RuleID:         "security.toctou_check_then_act",
		RuleVersion:    "1",
		Kind:           "toctou_check_then_act",
		Category:       "security",
		Severity:       "medium",
		Confidence:     confidence,
		Path:           path,
		Subject:        finding.Name,
		Location:       finding.Location,
		Evidence:       finding.Evidence,
		WhyItMatters:   toctouCheckThenActWhyItMatters,
		Recommendation: recommendation,
		SuggestedSkill: finding.SuggestedSkill,
		Provenance: Provenance{
			Producer:    "semantics",
			FindingKind: "toctou_check_then_act",
		},
	}
}
