package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestHiddenInputMutation_OtherFindingKindsProduceNoSignals(t *testing.T) {
	head := &semantics.Result{
		Path:        "f.go",
		Language:    semantics.LanguageGo,
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "constructor_func", Name: "NewFoo"},
			{Kind: "pointer_return", Name: "NewFoo"},
			{Kind: "not_a_real_kind", Name: "Mystery"},
		},
	}

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "f.go", Status: "modified", Head: head},
		},
	})

	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals for non-mutates_input findings: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
}

func TestHiddenInputMutation_ConfidenceDefaultsToMedium(t *testing.T) {
	tests := []struct {
		name       string
		confidence string
	}{
		{name: "empty", confidence: ""},
		{name: "unrecognized value", confidence: "bogus"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectHiddenInputMutationConfidence(t, tt.confidence, "medium")
		})
	}
}

func TestHiddenInputMutation_ConfidencePropagatesValidValues(t *testing.T) {
	for _, confidence := range []string{"low", "medium", "high"} {
		t.Run(confidence, func(t *testing.T) {
			expectHiddenInputMutationConfidence(t, confidence, Confidence(confidence))
		})
	}
}

func TestHiddenInputMutation_RecommendationDefaultsWhenEmpty(t *testing.T) {
	finding := semantics.Finding{Kind: "mutates_input", Recommendation: ""}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Recommendation != defaultHiddenInputMutationRecommendation {
		t.Errorf("newHiddenInputMutationSignal with empty Recommendation: got %q, want the rule default %q", signal.Recommendation, defaultHiddenInputMutationRecommendation)
	}
}

func TestHiddenInputMutation_RecommendationPreservedWhenPresent(t *testing.T) {
	finding := semantics.Finding{Kind: "mutates_input", Recommendation: "custom text"}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Recommendation != "custom text" {
		t.Errorf("newHiddenInputMutationSignal with Recommendation=%q: got %q, want it preserved verbatim", finding.Recommendation, signal.Recommendation)
	}
}

func expectHiddenInputMutationConfidence(t *testing.T, confidence string, want Confidence) {
	t.Helper()
	finding := semantics.Finding{Kind: "mutates_input", Confidence: confidence}
	signal := newHiddenInputMutationSignal("f.go", finding)
	if signal.Confidence != want {
		t.Errorf("newHiddenInputMutationSignal with Confidence=%q: got Confidence %q, want %q", confidence, signal.Confidence, want)
	}
}
