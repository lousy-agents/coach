package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestHiddenInputMutation_RecommendationPreservedWhenPresent(t *testing.T) {
	finding := semantics.Finding{Kind: "mutates_input", Recommendation: "custom text"}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Recommendation != "custom text" {
		t.Errorf("newHiddenInputMutationSignal with Recommendation=%q: got %q, want it preserved verbatim", finding.Recommendation, signal.Recommendation)
	}
}
