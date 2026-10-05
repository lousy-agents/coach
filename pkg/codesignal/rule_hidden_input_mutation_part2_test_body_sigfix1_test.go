package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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
