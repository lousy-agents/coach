package semantics

import (
	"testing"
)

// Story 1 (CWE-367, GitHub issue #177), positive case: a bare
// existsSync(p) guard whose consequence block calls a matching fs act
// call on identical path text must yield exactly one
// toctou_check_then_act Finding, located at the act call (not the check
// call), with the documented Confidence/SuggestedSkill fields. Table
// covers every name in tsToctouActCallNames so deleting any one of them
// from that set fails a case here.
func TestComputeTSFeatures_TOCTOUCheckThenAct_PositiveFinding(t *testing.T) {
	tests := []struct {
		name        string
		actCallText string
	}{
		{name: "readFileSync", actCallText: `readFileSync(p, "utf8")`},
		{name: "writeFileSync", actCallText: `writeFileSync(p, "seed")`},
		{name: "appendFileSync", actCallText: `appendFileSync(p, "more")`},
		{name: "unlinkSync", actCallText: `unlinkSync(p)`},
		{name: "rmSync", actCallText: `rmSync(p)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := "function f(p: string) {\n\tif (existsSync(p)) {\n\t\t" + tt.actCallText + ";\n\t}\n}\n"
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
				t.Fatalf("computeTSFeatures for %q: got %d toctou_check_then_act findings (%+v), want exactly 1", source, len(got), findings)
			}
			f := got[0]
			if f.Kind != "toctou_check_then_act" {
				t.Errorf("Finding.Kind = %q, want %q", f.Kind, "toctou_check_then_act")
			}
			if f.Confidence != "medium" {
				t.Errorf("Finding.Confidence = %q, want %q", f.Confidence, "medium")
			}
			if f.SuggestedSkill != "find-bugs" {
				t.Errorf("Finding.SuggestedSkill = %q, want %q", f.SuggestedSkill, "find-bugs")
			}
			gotText := source[f.Location.StartByte:f.Location.EndByte]
			if gotText != tt.actCallText {
				t.Errorf("Finding.Location text = %q, want %q (Location must point at the act call, not the check call)", gotText, tt.actCallText)
			}
		})
	}
}

// Story 1, exclusion cases: none of these guarded-body shapes match the
// narrow "bare existsSync(path) call gates a matching fs act call on the
// identical path text" pattern, so each must yield zero
// toctou_check_then_act findings.
func TestComputeTSFeatures_TOCTOUCheckThenAct_ExcludedCases(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "path text differs between check and act",
			source: `function f(a: string, b: string) {
	if (existsSync(a)) {
		readFileSync(b);
	}
}
`,
		},
		{
			name: "no act call anywhere in the guarded body",
			source: `function f(p: string) {
	if (existsSync(p)) {
		doSomethingElse();
	}
}
`,
		},
		{
			name: "condition wrapped in &&",
			source: `function f(p: string, other: boolean) {
	if (existsSync(p) && other) {
		readFileSync(p);
	}
}
`,
		},
		{
			name: "existsSync appears only inside a ternary expression condition, not an if/while",
			source: `function f(p: string) {
	const x = existsSync(p) ? readFileSync(p) : undefined;
	return x;
}
`,
		},
		{
			name: "existsSync gates a for loop condition, not an if/while",
			source: `function f(p: string) {
	for (; existsSync(p); ) {
		readFileSync(p);
	}
}
`,
		},
		{
			name: "negated check gates the does-not-exist branch, not the exists branch",
			source: `function f(p: string) {
	if (!existsSync(p)) {
		readFileSync(p);
	}
}
`,
		},
		{
			name: "act call sits in the else branch, which is not gated by the check",
			source: `function f(p: string) {
	if (existsSync(p)) {
		log(p);
	} else {
		writeFileSync(p, "seed");
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			_, findings := computeTSFeatures(root, []byte(tt.source))
			for _, f := range findings {
				if f.Kind == "toctou_check_then_act" {
					t.Fatalf("computeTSFeatures for %q: got toctou_check_then_act finding %+v, want none", tt.source, f)
				}
			}
		})
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
