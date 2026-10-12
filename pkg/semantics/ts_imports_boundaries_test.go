package semantics

import (
	"testing"
)

// Smoke check (not itself an AC): the TS import query must compile against
// the real TypeScript grammar.
func TestTSImportQuerySource_CompilesAgainstTSGrammar(t *testing.T) {
	query, queryErr := languageRegistry[LanguageTypeScript].engineLang.NewQuery(tsImportQuerySource)
	if queryErr != nil {
		t.Fatalf("compiling TS import query %q against the TypeScript grammar: got err %v, want nil", tsImportQuerySource, queryErr)
	}
	defer query.Close()
}

// AC-R3.3: re-exports, require(...), and dynamic import(...) are all out
// of scope for v1 and must produce no ImportFeature.
func TestExtractTSImports_OutOfScopeFormsProduceNoImports(t *testing.T) {
	tests := []tsImportSourceCase{
		{name: "re-export", source: `export { X } from "./x";`},
		{name: "require", source: `const y = require("./y");`},
		{name: "dynamic import", source: `async function f() { await import("./z"); }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectNoTSImports(t, tt)
		})
	}
}

type tsImportSourceCase struct {
	name   string
	source string
}

func expectNoTSImports(t *testing.T, tt tsImportSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	imports, err := extractTSImports(languageRegistry[LanguageTypeScript].engineLang, root, []byte(tt.source))
	if err != nil {
		t.Fatalf("extractTSImports for %s %q: got err %v, want nil", tt.name, tt.source, err)
	}
	if len(imports) != 0 {
		t.Errorf("extractTSImports for %s %q: got %d imports (%+v), want 0 (out of scope for v1)", tt.name, tt.source, len(imports), imports)
	}
}

// AC-R3.4: a file with no imports yields a nil/empty Imports slice.
func TestExtractTSImports_NoImportsYieldsEmptySlice(t *testing.T) {
	source := []byte(`const x = 1;`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	imports, err := extractTSImports(languageRegistry[LanguageTypeScript].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractTSImports for %q: got err %v, want nil", source, err)
	}
	if len(imports) != 0 {
		t.Errorf("extractTSImports for %q: got %d imports (%+v), want 0", source, len(imports), imports)
	}
}

// Regression guard: a Tree-sitter Query is compiled against, and only
// matches node-type IDs from, the specific Language it was built with.
// TypeScript and TSX share identical node kind name strings but different
// internal type IDs, so a query compiled against the TypeScript grammar
// silently matches nothing against a tree parsed with the TSX grammar (and
// vice versa) -- confirmed empirically before extractTSXImports was split
// out as its own grammar-parameterized closure. This test locks in that
// extractTSXImports (not extractTSImports) is what must be registered for
// LanguageTSX.
func TestExtractTSImports_DoesNotMatchAgainstATSXParsedTree(t *testing.T) {
	source := []byte(`import React from "react";`)
	root, closeTree := mustParseTSX(t, source)
	defer closeTree()

	imports, err := extractTSImports(languageRegistry[LanguageTypeScript].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractTSImports against a TSX-parsed tree %q: got err %v, want nil", source, err)
	}
	if len(imports) != 0 {
		t.Fatalf("extractTSImports (compiled against the TypeScript grammar) against a TSX-parsed tree %q: got %d imports (%+v), want 0 -- grammar/query mismatch should yield no matches, not wrong data", source, len(imports), imports)
	}
}

// AC-R2.3 (import half): extractTSXImports must extract imports from a
// TSX-parsed tree the same way extractTSImports does for TS, proving the
// grammar-parameterized sharing works across both grammars.
func TestExtractTSXImports_ExtractsImportsFromTSXParsedTree(t *testing.T) {
	source := []byte(`import React from "react";
const App = () => <div>hi</div>;
`)
	root, closeTree := mustParseTSX(t, source)
	defer closeTree()

	imports, err := extractTSXImports(languageRegistry[LanguageTSX].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractTSXImports for %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractTSXImports for %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	if imports[0].Path != "react" {
		t.Errorf("extractTSXImports for %q: Path = %q, want %q", source, imports[0].Path, "react")
	}
}
