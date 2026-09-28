package codesignal

import (
	"context"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestInputImmutability_BuildDoesNotMutateInput(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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
	for i, fc := range input.Files {
		cp := fc
		cp.ChangedRanges = append([]LineRange(nil), fc.ChangedRanges...)
		filesSnapshot[i] = cp
	}
	diagnosticsSnapshot := append([]Diagnostic(nil), input.Diagnostics...)
	diagnosticsLen, diagnosticsCap := len(input.Diagnostics), cap(input.Diagnostics)
	var diagnosticsFirstAddr *Diagnostic
	if len(input.Diagnostics) > 0 {
		diagnosticsFirstAddr = &input.Diagnostics[0]
	}

	report, err := b.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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

	if len(input.Files) != len(filesSnapshot) {
		t.Fatalf("Input.Files length changed: got %d, want %d", len(input.Files), len(filesSnapshot))
	}
	for i, fc := range input.Files {
		want := filesSnapshot[i]
		if fc.Path != want.Path || fc.Status != want.Status {
			t.Errorf("Files[%d] Path/Status mutated: got %+v, want %+v", i, fc, want)
		}
		if !reflect.DeepEqual(fc.ChangedRanges, want.ChangedRanges) {
			t.Errorf("Files[%d].ChangedRanges mutated: got %+v, want %+v", i, fc.ChangedRanges, want.ChangedRanges)
		}
	}

	if len(input.Diagnostics) != diagnosticsLen || cap(input.Diagnostics) != diagnosticsCap {
		t.Errorf("Input.Diagnostics len/cap changed: got %d/%d, want %d/%d", len(input.Diagnostics), cap(input.Diagnostics), diagnosticsLen, diagnosticsCap)
	}
	if len(input.Diagnostics) > 0 && &input.Diagnostics[0] != diagnosticsFirstAddr {
		t.Errorf("Input.Diagnostics backing array changed (first element address differs)")
	}
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
