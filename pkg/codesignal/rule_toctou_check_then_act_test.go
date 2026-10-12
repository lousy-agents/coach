package codesignal

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestTOCTOUCheckThenAct_ConfidenceDefaultsToMedium(t *testing.T) {
	tests := []struct {
		name       string
		confidence string
	}{
		{name: "empty", confidence: ""},
		{name: "unrecognized value", confidence: "bogus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectTOCTOUConfidence(t, tt.confidence, "medium")
		})
	}
}

func TestTOCTOUCheckThenAct_ConfidencePropagatesValidValues(t *testing.T) {
	for _, confidence := range []string{"low", "medium", "high"} {
		t.Run(confidence, func(t *testing.T) {
			expectTOCTOUConfidence(t, confidence, Confidence(confidence))
		})
	}
}

func TestTOCTOUCheckThenAct_RecommendationDefaultsWhenEmpty(t *testing.T) {
	finding := semantics.Finding{Kind: "toctou_check_then_act", Recommendation: ""}

	signal := newTOCTOUCheckThenActSignal("f.ts", finding)

	if signal.Recommendation != defaultTOCTOUCheckThenActRecommendation {
		t.Errorf("newTOCTOUCheckThenActSignal with empty Recommendation: got %q, want the rule default %q", signal.Recommendation, defaultTOCTOUCheckThenActRecommendation)
	}
}

func TestTOCTOUCheckThenAct_RecommendationPreservedWhenPresent(t *testing.T) {
	finding := semantics.Finding{Kind: "toctou_check_then_act", Recommendation: "custom text"}

	signal := newTOCTOUCheckThenActSignal("f.ts", finding)

	if signal.Recommendation != "custom text" {
		t.Errorf("newTOCTOUCheckThenActSignal with Recommendation=%q: got %q, want it preserved verbatim", finding.Recommendation, signal.Recommendation)
	}
}

func TestTOCTOUCheckThenAct_WhyItMattersReferencesCWE367(t *testing.T) {
	if !strings.Contains(toctouCheckThenActWhyItMatters, "CWE-367") {
		t.Errorf("toctouCheckThenActWhyItMatters: got %q, want it to reference CWE-367", toctouCheckThenActWhyItMatters)
	}
}

func TestTOCTOUCheckThenAct_NotDensityGated(t *testing.T) {
	if gatedFindingKinds["toctou_check_then_act"] {
		t.Errorf("gatedFindingKinds[\"toctou_check_then_act\"]: got true, want false (this rule must not be density-gated)")
	}
}

func expectTOCTOUConfidence(t *testing.T, confidence string, want Confidence) {
	t.Helper()
	finding := semantics.Finding{Kind: "toctou_check_then_act", Confidence: confidence}
	signal := newTOCTOUCheckThenActSignal("f.ts", finding)
	if signal.Confidence != want {
		t.Errorf("newTOCTOUCheckThenActSignal with Confidence=%q: got Confidence %q, want %q", confidence, signal.Confidence, want)
	}
}
