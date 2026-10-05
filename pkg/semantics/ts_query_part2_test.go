package semantics

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// mustParseTS parses source as TypeScript and returns its root node plus a
// cleanup function that closes the underlying tree.
func mustParseTS(t *testing.T, source []byte) (engine.Node, func()) {
	t.Helper()

	sp := newSyntaxParser()
	tree, err := sp.parse(context.Background(), source, LanguageTypeScript)
	if err != nil {
		t.Fatalf("parsing TS source %q: got err %v, want nil", source, err)
	}
	return tree.RootNode(), tree.Close
}

// mustParseTSX parses source as TSX and returns its root node plus a
// cleanup function that closes the underlying tree.
func mustParseTSX(t *testing.T, source []byte) (engine.Node, func()) {
	t.Helper()

	sp := newSyntaxParser()
	tree, err := sp.parse(context.Background(), source, LanguageTSX)
	if err != nil {
		t.Fatalf("parsing TSX source %q: got err %v, want nil", source, err)
	}
	return tree.RootNode(), tree.Close
}

// Smoke check (not itself an AC): the TS import query must compile against
// the real TypeScript grammar.
func TestTSImportQuerySource_CompilesAgainstTSGrammar(t *testing.T) {
	query, queryErr := languageRegistry[LanguageTypeScript].engineLang.NewQuery(tsImportQuerySource)
	if queryErr != nil {
		t.Fatalf("compiling TS import query %q against the TypeScript grammar: got err %v, want nil", tsImportQuerySource, queryErr)
	}
	defer query.Close()
}
