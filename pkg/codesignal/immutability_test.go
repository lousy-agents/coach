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

type sigTestInputImmutabilityBuildDoesNotMutateInputS1 struct {
	fc FileChange
	i  int
	t  *testing.
		T
	want FileChange
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS1) call() {

	if sigRecv.fc.Path != sigRecv.want.Path || sigRecv.fc.Status != sigRecv.want.Status {
		sigRecv.t.
			Errorf("Files[%d] Path/Status mutated: got %+v, want %+v", sigRecv.i, sigRecv.fc, sigRecv.want)
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS2 struct {
	fc FileChange
	i  int
	t  *testing.
		T
	want FileChange
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS2) call() {

	if !reflect.DeepEqual(sigRecv.fc.ChangedRanges, sigRecv.want.ChangedRanges) {
		sigRecv.t.
			Errorf("Files[%d].ChangedRanges mutated: got %+v, want %+v", sigRecv.i, sigRecv.fc.ChangedRanges, sigRecv.want.ChangedRanges)
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS10 struct {
	err error
	t   *testing.
		T
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS10) call() {

	if sigRecv.err != nil {
		sigRecv.t.
			Fatalf("New: %v", sigRecv.err)
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS7 struct {
	filesSnapshot []FileChange
	input         Input
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS7) call() {

	for i, fc := range sigRecv.input.Files {
		cp := fc
		cp.ChangedRanges = append([]LineRange(nil), fc.ChangedRanges...)
		sigRecv.filesSnapshot[i] = cp
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS11 struct {
	diagnosticsFirstAddr **Diagnostic
	input                Input
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS11) call() {

	if len(sigRecv.input.Diagnostics) > 0 {
		*sigRecv.diagnosticsFirstAddr = &sigRecv.input.Diagnostics[0]
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS13 struct {
	err error
	t   *testing.
		T
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS13) call() {

	if sigRecv.err != nil {
		sigRecv.t.
			Fatalf("Build: %v", sigRecv.err)
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS15 struct {
	filesSnapshot []FileChange
	input         Input
	t             *testing.
			T
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS15) call() {

	if len(sigRecv.input.Files) != len(sigRecv.filesSnapshot) {
		sigRecv.t.
			Fatalf("Input.Files length changed: got %d, want %d", len(sigRecv.input.Files), len(sigRecv.filesSnapshot))
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS16 struct {
	filesSnapshot []FileChange
	input         Input
	t             *testing.
			T
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS16) call() {

	for i, fc := range sigRecv.input.Files {
		want := sigRecv.filesSnapshot[i]
		(&sigTestInputImmutabilityBuildDoesNotMutateInputS1{fc: fc, i: i, t: sigRecv.t, want: want}).call()
		(&sigTestInputImmutabilityBuildDoesNotMutateInputS2{fc: fc, i: i, t: sigRecv.t, want: want}).call()

	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS17 struct {
	diagnosticsCap int
	diagnosticsLen int
	input          Input
	t              *testing.
			T
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS17) call() {

	if len(sigRecv.input.Diagnostics) != sigRecv.diagnosticsLen || cap(sigRecv.input.Diagnostics) != sigRecv.diagnosticsCap {
		sigRecv.t.
			Errorf("Input.Diagnostics len/cap changed: got %d/%d, want %d/%d", len(sigRecv.input.Diagnostics), cap(sigRecv.input.Diagnostics), sigRecv.diagnosticsLen, sigRecv.diagnosticsCap)
	}
}

type sigTestInputImmutabilityBuildDoesNotMutateInputS18 struct {
	diagnosticsFirstAddr *Diagnostic
	input                Input
	t                    *testing.
				T
}

func (sigRecv *sigTestInputImmutabilityBuildDoesNotMutateInputS18) call() {

	if len(sigRecv.input.Diagnostics) > 0 && &sigRecv.input.Diagnostics[0] != sigRecv.diagnosticsFirstAddr {
		sigRecv.t.
			Errorf("Input.Diagnostics backing array changed (first element address differs)")
	}
}
