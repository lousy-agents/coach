package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_ruleHiddenInputMutationTest_77(t *testing.T, tt struct {
	name       string
	confidence string
}) {
	finding := semantics.Finding{Kind: "mutates_input", Confidence: tt.confidence}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Confidence != Confidence("medium") {
		t.Errorf("newHiddenInputMutationSignal with Confidence=%q: got Confidence %q, want %q", tt.confidence, signal.Confidence, "medium")
	}
}

func body_ruleHiddenInputMutationTest_91(t *testing.T, confidence string) {
	finding := semantics.Finding{Kind: "mutates_input", Confidence: confidence}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Confidence != Confidence(confidence) {
		t.Errorf("newHiddenInputMutationSignal with Confidence=%q: got Confidence %q, want %q", confidence, signal.Confidence, confidence)
	}
}
