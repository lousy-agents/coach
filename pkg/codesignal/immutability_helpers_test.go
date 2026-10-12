package codesignal

import (
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func snapshotResult(r *semantics.Result) *semantics.Result {
	if r == nil {
		return nil
	}
	cp := *r
	cp.SyntaxErrors = append([]semantics.SyntaxIssue(nil), r.SyntaxErrors...)
	cp.Imports = append([]semantics.ImportFeature(nil), r.Imports...)
	cp.Findings = append([]semantics.Finding(nil), r.Findings...)
	return &cp
}

// fileResultSnapshot keeps both the caller's Base/Head pointers and deep
// copies of what they point at, so a check can tell a replaced pointer from
// a pointee mutated in place.
type fileResultSnapshot struct {
	basePtr, headPtr   *semantics.Result
	baseSnap, headSnap *semantics.Result
}

func snapshotFileResults(files []FileChange) []fileResultSnapshot {
	snapshots := make([]fileResultSnapshot, len(files))
	for i, fc := range files {
		snapshots[i] = fileResultSnapshot{
			basePtr:  fc.Base,
			headPtr:  fc.Head,
			baseSnap: snapshotResult(fc.Base),
			headSnap: snapshotResult(fc.Head),
		}
	}
	return snapshots
}

func expectFileResultsUnchanged(t *testing.T, files []FileChange, snapshots []fileResultSnapshot) {
	t.Helper()
	for i, fc := range files {
		want := snapshots[i]
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
}

// diagnosticsSnapshot records the caller's Diagnostics slice header as well
// as its contents: a changed len/cap or first-element address means Build
// appended into, or reallocated, the caller's backing array.
type diagnosticsSnapshot struct {
	contents         []Diagnostic
	length, capacity int
	first            *Diagnostic
}

func snapshotDiagnostics(diagnostics []Diagnostic) diagnosticsSnapshot {
	snapshot := diagnosticsSnapshot{
		contents: append([]Diagnostic(nil), diagnostics...),
		length:   len(diagnostics),
		capacity: cap(diagnostics),
	}
	if len(diagnostics) > 0 {
		snapshot.first = &diagnostics[0]
	}
	return snapshot
}

func expectDiagnosticsUnchanged(t *testing.T, diagnostics []Diagnostic, want diagnosticsSnapshot) {
	t.Helper()
	if len(diagnostics) != want.length || cap(diagnostics) != want.capacity {
		t.Errorf("Input.Diagnostics len/cap changed: got %d/%d, want %d/%d", len(diagnostics), cap(diagnostics), want.length, want.capacity)
	}
	if len(diagnostics) > 0 && &diagnostics[0] != want.first {
		t.Errorf("Input.Diagnostics backing array changed (first element address differs)")
	}
	if !reflect.DeepEqual(diagnostics, want.contents) {
		t.Errorf("Input.Diagnostics contents mutated: got %+v, want %+v", diagnostics, want.contents)
	}
}
