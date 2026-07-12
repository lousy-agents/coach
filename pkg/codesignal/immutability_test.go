package codesignal

import (
	"context"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// richImmutabilityInput builds an Input with multiple FileChange entries,
// populated Base/Head *semantics.Result pointers (Findings, Imports,
// SyntaxErrors, Metrics all set), ChangedRanges, and Input.Diagnostics --
// everything Build reads from that could plausibly be mutated in place.
func richImmutabilityInput() Input {
	return Input{
		Scope: Scope{Repository: "example/repo", Revision: "rev1", Base: "main"},
		Diagnostics: []Diagnostic{
			{Path: "z.go", Kind: "custom", Message: "hello"},
		},
		Files: []FileChange{
			{
				Path:   "a.go",
				Status: "modified",
				Base: &semantics.Result{
					Path:        "a.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Imports: []semantics.ImportFeature{
						{Path: "fmt", Location: semantics.Location{StartRow: 1}},
					},
					Metrics: semantics.StructuralMetrics{Ifs: 3, Functions: 2},
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}, Evidence: "x = 1"},
						{Kind: "mutates_input", Name: "GoneNow", Location: semantics.Location{StartRow: 2}, Evidence: "y = 2"},
					},
				},
				Head: &semantics.Result{
					Path:        "a.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Imports: []semantics.ImportFeature{
						{Path: "fmt", Location: semantics.Location{StartRow: 1}},
						{Path: "os", Location: semantics.Location{StartRow: 2}},
					},
					Metrics: semantics.StructuralMetrics{Ifs: 4, Functions: 3},
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}, Evidence: "x = 1"},
						{Kind: "mutates_input", Name: "NewOne", Location: semantics.Location{StartRow: 20}, Evidence: "z = 3"},
					},
				},
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 30}},
			},
			{
				Path:   "broken.go",
				Status: "modified",
				Head: &semantics.Result{
					Path:        "broken.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("syntax_errors"),
					SyntaxErrors: []semantics.SyntaxIssue{
						{Kind: "error", Location: semantics.Location{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 5}},
					},
				},
			},
			{
				Path:   "removed.go",
				Status: "removed",
				Base: &semantics.Result{
					Path:        "removed.go",
					Language:    semantics.LanguageGo,
					ParseStatus: semantics.ParseStatus("ok"),
					Findings: []semantics.Finding{
						{Kind: "mutates_input", Name: "Deleted", Location: semantics.Location{StartRow: 5}, Evidence: "w = 4"},
					},
				},
			},
		},
	}
}

// snapshotResult deep-copies a *semantics.Result's slice fields so later
// comparison against the live pointer can detect in-place mutation. A nil
// Result snapshots to nil.
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

// TestInputImmutability_BuildDoesNotMutateInput is a comprehensive
// input-immutability test going beyond the narrower
// TestBuild_DoesNotMutateInput (which only covers Input.Diagnostics): it
// snapshots every mutable part of a rich Input -- FileChange values
// (including ChangedRanges sub-slices), the pointee contents of every
// Base/Head *semantics.Result, and Input.Diagnostics -- calls Build, and
// asserts nothing was mutated in place, plus that the returned Report does
// not alias the same backing arrays as the input.
func TestInputImmutability_BuildDoesNotMutateInput(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	input := richImmutabilityInput()

	// Snapshot Base/Head pointer identities and their pointee contents
	// before Build runs.
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

	// Snapshot Input.Files (value copies, including ChangedRanges
	// sub-slices) and Input.Diagnostics before Build runs.
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

	// 1. Every *semantics.Result pointer in Input.Files[i].Base/.Head is
	// unchanged: same pointer AND same field values.
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

	// 2. Input.Files slice contents (each FileChange value, including
	// ChangedRanges sub-slices) are unchanged.
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

	// 3. Input.Diagnostics is unchanged (length/cap/first-element-address
	// and contents).
	if len(input.Diagnostics) != diagnosticsLen || cap(input.Diagnostics) != diagnosticsCap {
		t.Errorf("Input.Diagnostics len/cap changed: got %d/%d, want %d/%d", len(input.Diagnostics), cap(input.Diagnostics), diagnosticsLen, diagnosticsCap)
	}
	if len(input.Diagnostics) > 0 && &input.Diagnostics[0] != diagnosticsFirstAddr {
		t.Errorf("Input.Diagnostics backing array changed (first element address differs)")
	}
	if !reflect.DeepEqual([]Diagnostic(input.Diagnostics), diagnosticsSnapshot) {
		t.Errorf("Input.Diagnostics contents mutated: got %+v, want %+v", input.Diagnostics, diagnosticsSnapshot)
	}

	// 4. The returned *Report's Signals/Diagnostics do not alias the same
	// backing arrays as the input: mutating report.Diagnostics[0].Message
	// after Build returns must not change Input.Diagnostics.
	if len(report.Diagnostics) == 0 {
		t.Fatalf("report.Diagnostics must be non-empty for this scenario to exercise aliasing")
	}
	report.Diagnostics[0].Message = "MUTATED-AFTER-BUILD"
	if !reflect.DeepEqual([]Diagnostic(input.Diagnostics), diagnosticsSnapshot) {
		t.Errorf("mutating report.Diagnostics[0] after Build changed Input.Diagnostics -- report and input alias the same backing array: got %+v, want unchanged %+v", input.Diagnostics, diagnosticsSnapshot)
	}
}
