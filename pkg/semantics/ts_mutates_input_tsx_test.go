package semantics

import (
	"fmt"
	"testing"
)

// TSX variant of the property-assignment case, proving mutates_input works
// against a TSX-parsed tree the same way it does for TS (same pattern as
// TestComputeTSFeatures_WorksOnTSXParsedTree for the metrics half).
func TestTSXMutatesInput_PropertyAssignment(t *testing.T) {
	source := `const Widget = (props) => {
	props.value = 1;
	return null;
};
`
	root, closeTree := mustParseTSX(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mustFindTSMutatesInput(t, source, findings)

	funcStart := len("const Widget = ")
	wantName := fmt.Sprintf("anonymous@%d:props", funcStart)
	if got.Name != wantName {
		t.Errorf("Finding.Name = %q, want %q", got.Name, wantName)
	}
	gotText := source[got.Location.StartByte:got.Location.EndByte]
	if gotText != "props.value" {
		t.Errorf("Finding.Location text = %q, want %q", gotText, "props.value")
	}
}

func TestTSXMutatesInput_CompoundAssignment(t *testing.T) {
	source := `const Widget = (props) => {
	props.value += 1;
	return null;
};
`
	root, closeTree := mustParseTSX(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "anonymous@15:props", "props.value")
	if got == nil {
		t.Fatalf("computeTSFeatures for TSX compound assignment %q: want mutates_input named %q with evidence %q, got %+v", source, "anonymous@15:props", "props.value", findings)
	}
}
