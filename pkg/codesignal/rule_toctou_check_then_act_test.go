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
			finding := semantics.Finding{Kind: "toctou_check_then_act", Confidence: tt.confidence}

			signal := newTOCTOUCheckThenActSignal("f.ts", finding)

			if signal.Confidence != Confidence("medium") {
				t.Errorf("newTOCTOUCheckThenActSignal with Confidence=%q: got Confidence %q, want %q", tt.confidence, signal.Confidence, "medium")
			}
		})
	}
}

func TestTOCTOUCheckThenAct_ConfidencePropagatesValidValues(t *testing.T) {
	for _, confidence := range []string{"low", "medium", "high"} {
		t.Run(confidence, func(t *testing.T) {
			finding := semantics.Finding{Kind: "toctou_check_then_act", Confidence: confidence}

			signal := newTOCTOUCheckThenActSignal("f.ts", finding)

			if signal.Confidence != Confidence(confidence) {
				t.Errorf("newTOCTOUCheckThenActSignal with Confidence=%q: got Confidence %q, want %q", confidence, signal.Confidence, confidence)
			}
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

func TestTOCTOUCheckThenAct_FieldsPassThroughFromFinding(t *testing.T) {
	finding := semantics.Finding{
		Kind:           "toctou_check_then_act",
		Name:           "readConfig",
		Location:       semantics.Location{StartRow: 4, EndRow: 4},
		Evidence:       "if (existsSync(path)) { readFileSync(path) }",
		SuggestedSkill: "find-bugs",
	}

	signal := newTOCTOUCheckThenActSignal("f.ts", finding)

	if signal.RuleID != "security.toctou_check_then_act" {
		t.Errorf("Signal.RuleID: got %q, want %q", signal.RuleID, "security.toctou_check_then_act")
	}
	if signal.RuleVersion != "1" {
		t.Errorf("Signal.RuleVersion: got %q, want %q", signal.RuleVersion, "1")
	}
	if signal.Kind != "toctou_check_then_act" {
		t.Errorf("Signal.Kind: got %q, want %q", signal.Kind, "toctou_check_then_act")
	}
	if signal.Category != Category("security") {
		t.Errorf("Signal.Category: got %q, want %q", signal.Category, "security")
	}
	if signal.Severity != Severity("medium") {
		t.Errorf("Signal.Severity: got %q, want %q", signal.Severity, "medium")
	}
	if signal.Path != "f.ts" {
		t.Errorf("Signal.Path: got %q, want %q", signal.Path, "f.ts")
	}
	if signal.Subject != finding.Name {
		t.Errorf("Signal.Subject: got %q, want %q", signal.Subject, finding.Name)
	}
	if signal.Location != finding.Location {
		t.Errorf("Signal.Location: got %+v, want %+v", signal.Location, finding.Location)
	}
	if signal.Evidence != finding.Evidence {
		t.Errorf("Signal.Evidence: got %q, want %q", signal.Evidence, finding.Evidence)
	}
	if signal.SuggestedSkill != finding.SuggestedSkill {
		t.Errorf("Signal.SuggestedSkill: got %q, want %q", signal.SuggestedSkill, finding.SuggestedSkill)
	}
	if signal.Provenance != (Provenance{Producer: "semantics", FindingKind: "toctou_check_then_act"}) {
		t.Errorf("Signal.Provenance: got %+v, want %+v", signal.Provenance, Provenance{Producer: "semantics", FindingKind: "toctou_check_then_act"})
	}
}

func TestTOCTOUCheckThenAct_NotDensityGated(t *testing.T) {
	if gatedFindingKinds["toctou_check_then_act"] {
		t.Errorf("gatedFindingKinds[\"toctou_check_then_act\"]: got true, want false (this rule must not be density-gated)")
	}
}
