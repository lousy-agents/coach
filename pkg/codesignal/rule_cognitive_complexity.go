package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

const cognitiveComplexityWhyItMatters = "High cognitive complexity means the control flow takes more mental effort to follow: nested branches, mixed logical sequences, and jumps compound so reviewers and authors miss paths."

const cognitiveComplexityRecommendation = "Extract nested branches into named helpers, replace nested conditionals with early returns or lookup tables, and simplify boolean expressions so each function stays linearly readable."

// signalsFromCognitiveComplexity maps over-threshold per-function Cognitive
// Complexity records to Signals. Empty or nil records yield no signals.
func signalsFromCognitiveComplexity(path string, records []semantics.FunctionCognitiveComplexity) []Signal {
	var signals []Signal
	for _, rec := range records {
		if !cognitiveComplexityRule.reaches(rec.Score) {
			continue
		}
		signals = append(signals, newCognitiveComplexitySignal(path, rec))
	}
	return signals
}

func newCognitiveComplexitySignal(path string, rec semantics.FunctionCognitiveComplexity) Signal {
	return Signal{
		RuleID:         cognitiveComplexityRule.ruleID,
		RuleVersion:    "1",
		Kind:           "cognitive_complexity",
		Category:       "complexity",
		Severity:       cognitiveComplexityRule.severity(rec.Score),
		Confidence:     "high",
		Path:           path,
		Subject:        rec.Name,
		Location:       rec.Location,
		Evidence:       cognitiveComplexityRule.evidence(rec.Score),
		WhyItMatters:   cognitiveComplexityWhyItMatters,
		Recommendation: cognitiveComplexityRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}
}
