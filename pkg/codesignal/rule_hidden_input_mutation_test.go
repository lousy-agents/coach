package codesignal

import (
	"context"
	"os"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// mustAnalyzeFixture reads a testdata fixture used by internal/jsbridge's
// parity suite and runs it through the real semantics.Analyzer, producing
// the actual Findings that back the hidden-input-mutation Signals under
// test here.
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

// TestHiddenInputMutation_EndToEndPerLanguage runs the real Go, TypeScript,
// and TSX mutates_input fixtures through Builder.Build and checks that at
// least one hidden_input_mutation Signal comes out with the fields the rule
// promises, for each language.
func TestHiddenInputMutation_EndToEndPerLanguage(t *testing.T) {
	tests := []struct {
		name           string
		srcPath        string
		resultPath     string
		lang           semantics.Language
		wantConfidence Confidence
	}{
		{
			name:           "go",
			srcPath:        "../../internal/jsbridge/testdata/parity/go_mutates_input.src",
			resultPath:     "example/mutate.go",
			lang:           semantics.LanguageGo,
			wantConfidence: "medium",
		},
		{
			name:           "typescript",
			srcPath:        "../../internal/jsbridge/testdata/parity/ts_mutates_input.src",
			resultPath:     "example/mutate.ts",
			lang:           semantics.LanguageTypeScript,
			wantConfidence: "medium",
		},
		{
			name:           "tsx",
			srcPath:        "../../internal/jsbridge/testdata/parity/tsx_mutates_input.src",
			resultPath:     "example/mutate.tsx",
			lang:           semantics.LanguageTSX,
			wantConfidence: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mustAnalyzeFixture(t, tt.srcPath, tt.resultPath, tt.lang)

			var wantFinding *semantics.Finding
			for i := range result.Findings {
				if result.Findings[i].Kind == "mutates_input" {
					wantFinding = &result.Findings[i]
					break
				}
			}
			if wantFinding == nil {
				t.Fatalf("fixture %s produced no mutates_input findings; test fixture assumption is stale", tt.srcPath)
			}

			b, err := New(Options{})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			report, err := b.Build(context.Background(), Input{
				Files: []FileChange{
					{
						Path:   tt.resultPath,
						Status: "modified",
						Head:   result,
					},
				},
			})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}

			var got *Signal
			for i := range report.Signals {
				if report.Signals[i].Subject == wantFinding.Name {
					got = &report.Signals[i]
					break
				}
			}
			if got == nil {
				t.Fatalf("Report.Signals does not contain a signal for finding %q: %+v", wantFinding.Name, report.Signals)
			}

			if got.RuleID != "state.hidden_input_mutation" {
				t.Errorf("Signal.RuleID: got %q, want %q", got.RuleID, "state.hidden_input_mutation")
			}
			if got.RuleVersion != "1" {
				t.Errorf("Signal.RuleVersion: got %q, want %q", got.RuleVersion, "1")
			}
			if got.Kind != "hidden_input_mutation" {
				t.Errorf("Signal.Kind: got %q, want %q", got.Kind, "hidden_input_mutation")
			}
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
		})
	}
}

// TestHiddenInputMutation_ConfidenceDefaultsToMedium proves that empty and
// unrecognized Finding.Confidence values both default to "medium" rather
// than being propagated verbatim.
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
			finding := semantics.Finding{Kind: "mutates_input", Confidence: tt.confidence}

			signal := newHiddenInputMutationSignal("f.go", finding)

			if signal.Confidence != Confidence("medium") {
				t.Errorf("newHiddenInputMutationSignal with Confidence=%q: got Confidence %q, want %q", tt.confidence, signal.Confidence, "medium")
			}
		})
	}
}

// TestHiddenInputMutation_ConfidencePropagatesValidValues proves that
// "low", "medium", and "high" Finding.Confidence values are used verbatim.
func TestHiddenInputMutation_ConfidencePropagatesValidValues(t *testing.T) {
	for _, confidence := range []string{"low", "medium", "high"} {
		t.Run(confidence, func(t *testing.T) {
			finding := semantics.Finding{Kind: "mutates_input", Confidence: confidence}

			signal := newHiddenInputMutationSignal("f.go", finding)

			if signal.Confidence != Confidence(confidence) {
				t.Errorf("newHiddenInputMutationSignal with Confidence=%q: got Confidence %q, want %q", confidence, signal.Confidence, confidence)
			}
		})
	}
}

// TestHiddenInputMutation_RecommendationDefaultsWhenEmpty proves that an
// empty Finding.Recommendation falls back to the rule's default text.
func TestHiddenInputMutation_RecommendationDefaultsWhenEmpty(t *testing.T) {
	finding := semantics.Finding{Kind: "mutates_input", Recommendation: ""}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Recommendation != defaultHiddenInputMutationRecommendation {
		t.Errorf("newHiddenInputMutationSignal with empty Recommendation: got %q, want the rule default %q", signal.Recommendation, defaultHiddenInputMutationRecommendation)
	}
}

// TestHiddenInputMutation_RecommendationPreservedWhenPresent proves that a
// non-empty Finding.Recommendation is used verbatim, not replaced by the
// rule default.
func TestHiddenInputMutation_RecommendationPreservedWhenPresent(t *testing.T) {
	finding := semantics.Finding{Kind: "mutates_input", Recommendation: "custom text"}

	signal := newHiddenInputMutationSignal("f.go", finding)

	if signal.Recommendation != "custom text" {
		t.Errorf("newHiddenInputMutationSignal with Recommendation=%q: got %q, want it preserved verbatim", finding.Recommendation, signal.Recommendation)
	}
}

// TestHiddenInputMutation_OtherFindingKindsProduceNoSignals proves that
// Build only maps "mutates_input" Findings to Signals: "constructor_func",
// "pointer_return", and an invented unknown kind must all be skipped.
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
