package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_ruleToctouCheckThenActPart2Test_20(t *testing.T, tt struct {
	name       string
	confidence string
}) {
	finding := semantics.Finding{Kind: "toctou_check_then_act", Confidence: tt.confidence}

	signal := newTOCTOUCheckThenActSignal("f.ts", finding)

	if signal.Confidence != Confidence("medium") {
		t.Errorf("newTOCTOUCheckThenActSignal with Confidence=%q: got Confidence %q, want %q", tt.confidence, signal.Confidence, "medium")
	}
}

func body_ruleToctouCheckThenActPart2Test_34(t *testing.T, confidence string) {
	finding := semantics.Finding{Kind: "toctou_check_then_act", Confidence: confidence}

	signal := newTOCTOUCheckThenActSignal("f.ts", finding)

	if signal.Confidence != Confidence(confidence) {
		t.Errorf("newTOCTOUCheckThenActSignal with Confidence=%q: got Confidence %q, want %q", confidence, signal.Confidence, confidence)
	}
}
