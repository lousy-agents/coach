package codesignal

import (
	"context"
	"os"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func mustAnalyzeFixture(t *testing.T, srcPath, resultPath string, lang semantics.Language) *semantics.Result {
	t.Helper()

	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Fatalf("reading fixture %s: %v", srcPath, err)
	}

	analyzer, err := semantics.NewAnalyzer(semantics.AnalyzerOptions{})
	if err != nil {
		t.Fatalf("semantics.NewAnalyzer: %v", err)
	}

	result, err := analyzer.AnalyzeBytes(context.Background(), semantics.FileInput{
		Path:     resultPath,
		Language: lang,
		Content:  content,
	})
	if err != nil {
		t.Fatalf("AnalyzeBytes(%s): %v", srcPath, err)
	}

	return result
}

func TestHiddenInputMutation_OtherFindingKindsProduceNoSignals(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "f.go", Status: "modified", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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
			body_ruleHiddenInputMutationTest_77(t, tt)
		})
	}
}

func TestHiddenInputMutation_ConfidencePropagatesValidValues(t *testing.T) {
	for _, confidence := range []string{"low", "medium", "high"} {
		t.Run(confidence, func(t *testing.T) {
			body_ruleHiddenInputMutationTest_91(t, confidence)
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
