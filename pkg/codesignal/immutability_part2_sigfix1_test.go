package codesignal

import (
	"reflect"
	"testing"
)

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
