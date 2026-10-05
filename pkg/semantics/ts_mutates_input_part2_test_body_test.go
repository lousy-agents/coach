package semantics

import (
	"testing"
)

type tsIndexAssignmentCase struct {
	name       string
	source     string
	wantParam  string
	wantSuffix string
}

func expectIndexAssignmentFinding(t *testing.T, tt tsIndexAssignmentCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	got := mustFindTSMutatesInput(t, tt.source, findings)

	if got.Name != "f:"+tt.wantParam {
		t.Errorf("Finding.Name = %q, want %q", got.Name, "f:"+tt.wantParam)
	}
	gotText := tt.source[got.Location.StartByte:got.Location.EndByte]
	if gotText != tt.wantSuffix {
		t.Errorf("Finding.Location text = %q, want %q", gotText, tt.wantSuffix)
	}
}
