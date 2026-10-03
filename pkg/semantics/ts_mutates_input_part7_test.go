package semantics

import (
	"fmt"
	"testing"
)

// mutatesInputFindingNamed returns the first mutates_input Finding in
// findings matching both name and evidence, or nil if none matches.
func mutatesInputFindingNamed(findings []Finding, name, evidence string) *Finding {
	for i := range findings {
		if findings[i].Kind == "mutates_input" && findings[i].Name == name && findings[i].Evidence == evidence {
			return &findings[i]
		}
	}
	return nil
}

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

func TestTSMutatesInput_BracketNotationMutatingMethodCall(t *testing.T) {
	source := `function f(arr) {
	arr["push"](1);
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))

	got := mutatesInputFindingNamed(findings, "f:arr", `arr["push"]`)
	if got == nil {
		t.Fatalf("computeTSFeatures for bracket-notation mutating method call %q: want a mutates_input finding named %q with evidence %q, got %+v", source, "f:arr", `arr["push"]`, findings)
	}
}

func TestTSMutatesInput_VarParameterRebindDoesNotSuppressEarlierMutation(t *testing.T) {
	source := `function f(p) {
	p.x = 1;
	var p = {};
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input before same-name var rebind, got %+v", source, findings)
	}
}

func TestTSMutatesInput_NoInitializerVarParameterDoesNotSuppressLaterMutation(t *testing.T) {
	source := `function f(p) {
	var p;
	p.x = 1;
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))
	got := mutatesInputFindingNamed(findings, "f:p", "p.x")
	if got == nil {
		t.Fatalf("computeTSFeatures for %q: want mutates_input after no-initializer var parameter declaration, got %+v", source, findings)
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
