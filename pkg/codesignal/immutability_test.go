package codesignal

import (
	"context"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestInputImmutability_BuildDoesNotMutateInput(t *testing.T) {
	input := richImmutabilityInput()
	results := snapshotFileResults(input.Files)
	files := snapshotFileChanges(input.Files)
	diagnostics := snapshotDiagnostics(input.Diagnostics)

	report := mustBuild(t, Options{IncludeResolved: true}, input)

	expectFileResultsUnchanged(t, input.Files, results)
	expectFileChangesUnchanged(t, input.Files, files)
	expectDiagnosticsUnchanged(t, input.Diagnostics, diagnostics)

	if len(report.Diagnostics) == 0 {
		t.Fatalf("report.Diagnostics must be non-empty for this scenario to exercise aliasing")
	}
	report.Diagnostics[0].Message = "MUTATED-AFTER-BUILD"
	if !reflect.DeepEqual([]Diagnostic(input.Diagnostics), diagnostics.contents) {
		t.Errorf("mutating report.Diagnostics[0] after Build changed Input.Diagnostics -- report and input alias the same backing array: got %+v, want unchanged %+v", input.Diagnostics, diagnostics.contents)
	}
}

func TestBuild_DoesNotMutateInput(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	callerDiagnostics := []Diagnostic{
		{Path: "z.go", Kind: "custom", Message: "hello"},
	}
	input := Input{
		Files: []FileChange{
			{Path: "m.go", Base: &semantics.Result{Path: "wrong.go"}},
		},
		Diagnostics: callerDiagnostics,
	}

	if _, err := b.Build(context.Background(), input); err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}

	if len(callerDiagnostics) != 1 {
		t.Errorf("caller's Diagnostics slice must not be mutated in place; got length %d, want 1", len(callerDiagnostics))
	}
	if callerDiagnostics[0].Path != "z.go" || callerDiagnostics[0].Kind != "custom" {
		t.Errorf("caller's Diagnostics slice contents must not be mutated; got %+v", callerDiagnostics[0])
	}
}

func TestLifecycle_ClassifyFileSignals_DoesNotMutateInputSlices(t *testing.T) {
	head := []Signal{sig("r", "f.go", "M", "ev", 1, 0)}
	base := []Signal{sig("r", "f.go", "M", "ev", 1, 0)}

	headCopy := append([]Signal(nil), head...)
	baseCopy := append([]Signal(nil), base...)

	classifyFileSignals(true, head, base, "unknown")

	if head[0].Lifecycle != headCopy[0].Lifecycle || head[0].Fingerprint != headCopy[0].Fingerprint {
		t.Errorf("classifyFileSignals must not mutate the caller's headSignals slice in place: got %+v, want %+v", head[0], headCopy[0])
	}
	if base[0].Lifecycle != baseCopy[0].Lifecycle || base[0].Fingerprint != baseCopy[0].Fingerprint {
		t.Errorf("classifyFileSignals must not mutate the caller's baseSignals slice in place: got %+v, want %+v", base[0], baseCopy[0])
	}
}
