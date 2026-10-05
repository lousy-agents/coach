package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestHiddenInputMutation_EndToEndPerLanguage(t *testing.T) {
	tests := []hiddenInputMutationFixture{
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
			expectFixtureRaisesHiddenInputMutation(t, tt)
		})
	}
}

// expectFixtureRaisesHiddenInputMutation analyzes the fixture source through
// pkg/semantics, builds a report from it, and checks the signal raised for
// its first mutates_input finding.
func expectFixtureRaisesHiddenInputMutation(t *testing.T, tt hiddenInputMutationFixture) {
	t.Helper()
	result := mustAnalyzeFixture(t, tt.srcPath, tt.resultPath, tt.lang)

	wantFinding, ok := firstMutatesInputFinding(result)
	if !ok {
		t.Fatalf("fixture %s produced no mutates_input findings; test fixture assumption is stale", tt.srcPath)
	}

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{
				Path:   tt.resultPath,
				Status: "modified",
				Head:   result,
			},
		},
	})

	got, ok := signalWithSubject(report.Signals, wantFinding.Name)
	if !ok {
		t.Fatalf("Report.Signals does not contain a signal for finding %q: %+v", wantFinding.Name, report.Signals)
	}
	expectHiddenInputMutationSignal(t, got, tt, wantFinding)
}

func expectHiddenInputMutationSignal(t *testing.T, got Signal, tt hiddenInputMutationFixture, wantFinding semantics.Finding) {
	t.Helper()
	fields := []struct {
		name      string
		got, want string
	}{
		{"RuleID", got.RuleID, "state.hidden_input_mutation"},
		{"RuleVersion", got.RuleVersion, "1"},
		{"Kind", got.Kind, "hidden_input_mutation"},
		{"Category", string(got.Category), "state_management"},
		{"Severity", string(got.Severity), "medium"},
		{"Confidence", string(got.Confidence), string(tt.wantConfidence)},
		{"Path", got.Path, tt.resultPath},
		{"Subject", got.Subject, wantFinding.Name},
		{"Evidence", got.Evidence, wantFinding.Evidence},
		{"SuggestedSkill", got.SuggestedSkill, wantFinding.SuggestedSkill},
		{"Provenance.Producer", got.Provenance.Producer, "semantics"},
		{"Provenance.FindingKind", got.Provenance.FindingKind, "mutates_input"},
	}
	for _, f := range fields {
		if f.got != f.want {
			t.Errorf("Signal.%s: got %q, want %q", f.name, f.got, f.want)
		}
	}
	if wantFinding.Recommendation != "" && got.Recommendation != wantFinding.Recommendation {
		t.Errorf("Signal.Recommendation: got %q, want (preserved from finding) %q", got.Recommendation, wantFinding.Recommendation)
	}
	if got.WhyItMatters != hiddenInputMutationWhyItMatters {
		t.Errorf("Signal.WhyItMatters: got %q, want the deterministic rule-owned text", got.WhyItMatters)
	}
}

func signalWithSubject(signals []Signal, subject string) (Signal, bool) {
	for _, s := range signals {
		if s.Subject == subject {
			return s, true
		}
	}
	return Signal{}, false
}
