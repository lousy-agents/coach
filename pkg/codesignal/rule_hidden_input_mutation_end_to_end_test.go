package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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
			body_ruleHiddenInputMutationPart2Test_43(t, tt)
		})
	}
}

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

type sigbodyruleHiddenInputMutationPart2Test43S2 struct {
	result *semantics.
		Result
	wantFinding **semantics.
			Finding
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S2) call() {

	for i := range sigRecv.result.Findings {
		if sigRecv.result.Findings[i].Kind == "mutates_input" {
			*sigRecv.wantFinding = &sigRecv.result.Findings[i]
			break
		}
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S3 struct {
	t *testing.
		T
	tt struct {
		name           string
		srcPath        string
		resultPath     string
		lang           semantics.Language
		wantConfidence Confidence
	}
	wantFinding *semantics.
			Finding
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S3) call() {

	if sigRecv.wantFinding == nil {
		sigRecv.t.
			Fatalf("fixture %s produced no mutates_input findings; test fixture assumption is stale", sigRecv.tt.srcPath)
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S5 struct {
	err error
	t   *testing.
		T
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S5) call() {

	if sigRecv.err != nil {
		sigRecv.t.
			Fatalf("New: %v", sigRecv.err)
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S7 struct {
	err error
	t   *testing.
		T
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S7) call() {

	if sigRecv.err != nil {
		sigRecv.t.
			Fatalf("Build: %v", sigRecv.err)
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S9 struct {
	got         **Signal
	report      *Report
	wantFinding *semantics.
			Finding
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S9) call() {

	for i := range sigRecv.report.Signals {
		if sigRecv.report.Signals[i].Subject == sigRecv.wantFinding.Name {
			*sigRecv.got = &sigRecv.report.Signals[i]
			break
		}
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S10 struct {
	got    *Signal
	report *Report
	t      *testing.
		T
	wantFinding *semantics.
			Finding
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S10) call() {

	if sigRecv.got == nil {
		sigRecv.t.
			Fatalf("Report.Signals does not contain a signal for finding %q: %+v", sigRecv.wantFinding.Name, sigRecv.report.Signals)
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S11 struct {
	got *Signal
	t   *testing.
		T
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S11) call() {

	if sigRecv.got.RuleID != "state.hidden_input_mutation" {
		sigRecv.t.
			Errorf("Signal.RuleID: got %q, want %q", sigRecv.got.RuleID, "state.hidden_input_mutation")
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S12 struct {
	got *Signal
	t   *testing.
		T
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S12) call() {

	if sigRecv.got.RuleVersion != "1" {
		sigRecv.t.
			Errorf("Signal.RuleVersion: got %q, want %q", sigRecv.got.RuleVersion, "1")
	}
}

type sigbodyruleHiddenInputMutationPart2Test43S13 struct {
	got *Signal
	t   *testing.
		T
}

func (sigRecv *sigbodyruleHiddenInputMutationPart2Test43S13) call() {

	if sigRecv.got.Kind != "hidden_input_mutation" {
		sigRecv.t.
			Errorf("Signal.Kind: got %q, want %q", sigRecv.got.Kind, "hidden_input_mutation")
	}
}
