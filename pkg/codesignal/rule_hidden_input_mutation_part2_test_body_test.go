package codesignal

import (
	"context"

	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func body_ruleHiddenInputMutationPart2Test_43(t *testing.T, tt struct {
	name           string
	srcPath        string
	resultPath     string
	lang           semantics.Language
	wantConfidence Confidence
}) {
	result := mustAnalyzeFixture(t, tt.srcPath, tt.resultPath, tt.lang)

	var wantFinding *semantics.Finding
	(&sigbodyruleHiddenInputMutationPart2Test43S2{result: result, wantFinding: &wantFinding}).call()
	(&sigbodyruleHiddenInputMutationPart2Test43S3{t: t, tt: tt, wantFinding: wantFinding}).call()

	b, err := New(Options{})
	(&sigbodyruleHiddenInputMutationPart2Test43S5{err: err, t: t}).call()

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{
				Path:   tt.resultPath,
				Status: "modified",
				Head:   result,
			},
		},
	})
	(&sigbodyruleHiddenInputMutationPart2Test43S7{err: err, t: t}).call()

	var got *Signal
	(&sigbodyruleHiddenInputMutationPart2Test43S9{got: &got, report: report, wantFinding: wantFinding}).call()
	(&sigbodyruleHiddenInputMutationPart2Test43S10{got: got, report: report, t: t, wantFinding: wantFinding}).call()
	(&sigbodyruleHiddenInputMutationPart2Test43S11{got: got, t: t}).call()
	(&sigbodyruleHiddenInputMutationPart2Test43S12{got: got, t: t}).call()
	(&sigbodyruleHiddenInputMutationPart2Test43S13{got: got, t: t}).call()

	if got.Category != "state_management" {
		t.Errorf("Signal.Category: got %q, want %q", got.Category, "state_management")
	}
	if got.Severity != "medium" {
		t.Errorf("Signal.Severity: got %q, want %q", got.Severity, "medium")
	}
	if got.Confidence != tt.wantConfidence {
		t.Errorf("Signal.Confidence: got %q, want %q", got.Confidence, tt.wantConfidence)
	}
	if got.Path != tt.resultPath {
		t.Errorf("Signal.Path: got %q, want %q", got.Path, tt.resultPath)
	}
	if got.Subject != wantFinding.Name {
		t.Errorf("Signal.Subject: got %q, want %q", got.Subject, wantFinding.Name)
	}
	if got.Evidence != wantFinding.Evidence {
		t.Errorf("Signal.Evidence: got %q, want %q", got.Evidence, wantFinding.Evidence)
	}
	if wantFinding.Recommendation != "" && got.Recommendation != wantFinding.Recommendation {
		t.Errorf("Signal.Recommendation: got %q, want (preserved from finding) %q", got.Recommendation, wantFinding.Recommendation)
	}
	if got.WhyItMatters != hiddenInputMutationWhyItMatters {
		t.Errorf("Signal.WhyItMatters: got %q, want the deterministic rule-owned text", got.WhyItMatters)
	}
	if got.SuggestedSkill != wantFinding.SuggestedSkill {
		t.Errorf("Signal.SuggestedSkill: got %q, want %q", got.SuggestedSkill, wantFinding.SuggestedSkill)
	}
	if got.Provenance.Producer != "semantics" {
		t.Errorf("Signal.Provenance.Producer: got %q, want %q", got.Provenance.Producer, "semantics")
	}
	if got.Provenance.FindingKind != "mutates_input" {
		t.Errorf("Signal.Provenance.FindingKind: got %q, want %q", got.Provenance.FindingKind, "mutates_input")
	}
}
