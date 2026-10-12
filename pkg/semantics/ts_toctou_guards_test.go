package semantics

import (
	"testing"
)

// TSX variant of the positive TOCTOU case: computeTSFeatures must detect
// the same pattern in a TSX-parsed tree.
func TestComputeTSFeatures_TOCTOUCheckThenAct_WorksOnTSXParsedTree(t *testing.T) {
	source := []byte(`const Loader = (p: string) => {
	if (existsSync(p)) {
		const data = readFileSync(p, "utf8");
		return <div>{data}</div>;
	}
	return null;
};
`)
	root, closeTree := mustParseTSX(t, source)
	defer closeTree()

	_, findings := computeTSFeatures(root, source)

	var got []Finding
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("computeTSFeatures on TSX tree for %q: got %d toctou_check_then_act findings (%+v), want exactly 1", source, len(got), findings)
	}
	wantText := `readFileSync(p, "utf8")`
	gotText := string(source[got[0].Location.StartByte:got[0].Location.EndByte])
	if gotText != wantText {
		t.Errorf("computeTSFeatures on TSX tree for %q: Finding.Location text = %q, want %q", source, gotText, wantText)
	}
}

// Regression guard: two nested existsSync(p) guards on the identical path,
// both wrapping the same single act call, must still yield exactly one
// toctou_check_then_act Finding, not one per enclosing guard. The outer
// if's own checkTOCTOUCheckThenAct call resolves the readFileSync(p) act
// call by searching its whole guarded body (which includes the nested
// inner if), and the inner if's checkTOCTOUCheckThenAct call resolves the
// very same act call independently -- both emission paths must dedupe
// against the act call's Location.
func TestComputeTSFeatures_TOCTOUCheckThenAct_DedupesNestedGuardsOnSamePath(t *testing.T) {
	source := `function f(p: string) {
	if (existsSync(p)) {
		if (existsSync(p)) {
			readFileSync(p);
		}
	}
}
`
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))

	var got []Finding
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d toctou_check_then_act findings (%+v), want exactly 1 (nested guards on the same path must dedupe to a single Finding on the act call)", source, len(got), findings)
	}
}
