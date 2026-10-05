package codesignal

import (
	"context"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestInputImmutability_BuildDoesNotMutateInput(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS10{err: err, t: t}).call()

	input := richImmutabilityInput()

	type resultSnapshot struct {
		basePtr, headPtr   *semantics.Result
		baseSnap, headSnap *semantics.Result
	}
	perFile := make([]resultSnapshot, len(input.Files))
	for i, fc := range input.Files {
		perFile[i] = resultSnapshot{
			basePtr:  fc.Base,
			headPtr:  fc.Head,
			baseSnap: snapshotResult(fc.Base),
			headSnap: snapshotResult(fc.Head),
		}
	}

	filesSnapshot := make([]FileChange, len(input.Files))
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS7{filesSnapshot: filesSnapshot, input: input}).call()

	diagnosticsSnapshot := append([]Diagnostic(nil), input.Diagnostics...)
	diagnosticsLen, diagnosticsCap := len(input.Diagnostics), cap(input.Diagnostics)
	var diagnosticsFirstAddr *Diagnostic
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS11{diagnosticsFirstAddr: &diagnosticsFirstAddr, input: input}).call()

	report, err := b.Build(context.Background(), input)
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS13{err: err, t: t}).call()

	for i, fc := range input.Files {
		want := perFile[i]
		if fc.Base != want.basePtr {
			t.Errorf("Files[%d].Base pointer changed: got %p, want %p", i, fc.Base, want.basePtr)
		}
		if fc.Head != want.headPtr {
			t.Errorf("Files[%d].Head pointer changed: got %p, want %p", i, fc.Head, want.headPtr)
		}
		if !reflect.DeepEqual(fc.Base, want.baseSnap) {
			t.Errorf("Files[%d].Base pointee mutated in place: got %+v, want %+v", i, fc.Base, want.baseSnap)
		}
		if !reflect.DeepEqual(fc.Head, want.headSnap) {
			t.Errorf("Files[%d].Head pointee mutated in place: got %+v, want %+v", i, fc.Head, want.headSnap)
		}
	}
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS15{filesSnapshot: filesSnapshot, input: input, t: t}).call()
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS16{filesSnapshot: filesSnapshot, input: input, t: t}).call()
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS17{diagnosticsCap: diagnosticsCap, diagnosticsLen: diagnosticsLen, input: input, t: t}).call()
	(&sigTestInputImmutabilityBuildDoesNotMutateInputS18{diagnosticsFirstAddr: diagnosticsFirstAddr, input: input, t: t}).call()

	if !reflect.DeepEqual([]Diagnostic(input.Diagnostics), diagnosticsSnapshot) {
		t.Errorf("Input.Diagnostics contents mutated: got %+v, want %+v", input.Diagnostics, diagnosticsSnapshot)
	}

	if len(report.Diagnostics) == 0 {
		t.Fatalf("report.Diagnostics must be non-empty for this scenario to exercise aliasing")
	}
	report.Diagnostics[0].Message = "MUTATED-AFTER-BUILD"
	if !reflect.DeepEqual([]Diagnostic(input.Diagnostics), diagnosticsSnapshot) {
		t.Errorf("mutating report.Diagnostics[0] after Build changed Input.Diagnostics -- report and input alias the same backing array: got %+v, want unchanged %+v", input.Diagnostics, diagnosticsSnapshot)
	}
}
