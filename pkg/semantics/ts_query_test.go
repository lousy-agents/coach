package semantics

import (
	"context"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

// mustParseTS parses source as TypeScript and returns its root node plus a
// cleanup function that closes the underlying tree.
func mustParseTS(t *testing.T, source []byte) (*tree_sitter.Node, func()) {
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
func mustParseTSX(t *testing.T, source []byte) (*tree_sitter.Node, func()) {
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
	lang := tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())

	query, queryErr := tree_sitter.NewQuery(lang, tsImportQuerySource)
	if queryErr != nil {
		t.Fatalf("compiling TS import query %q against the TypeScript grammar: got err %v, want nil", tsImportQuerySource, queryErr)
	}
	defer query.Close()
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

	imports, err := extractTSImports(root, source)
	if err != nil {
		t.Fatalf("extractTSImports against a TSX-parsed tree %q: got err %v, want nil", source, err)
	}
	if len(imports) != 0 {
		t.Fatalf("extractTSImports (compiled against the TypeScript grammar) against a TSX-parsed tree %q: got %d imports (%+v), want 0 -- grammar/query mismatch should yield no matches, not wrong data", source, len(imports), imports)
	}
}

// AC-R3.1: every static import form (named, default, side-effect-only, and
// `import type`) must yield one ImportFeature each, in source order, all
// with empty Alias.
func TestExtractTSImports_AllStaticImportForms(t *testing.T) {
	source := []byte(`import { A } from "./a";
import B from './b';
import "./side";
import type { T } from "./t";
`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	imports, err := extractTSImports(root, source)
	if err != nil {
		t.Fatalf("extractTSImports for %q: got err %v, want nil", source, err)
	}

	wantPaths := []string{"./a", "./b", "./side", "./t"}
	if len(imports) != len(wantPaths) {
		t.Fatalf("extractTSImports for %q: got %d imports (%+v), want exactly %d", source, len(imports), imports, len(wantPaths))
	}
	for i, want := range wantPaths {
		if imports[i].Path != want {
			t.Errorf("extractTSImports for %q: imports[%d].Path = %q, want %q", source, i, imports[i].Path, want)
		}
		if imports[i].Alias != "" {
			t.Errorf("extractTSImports for %q: imports[%d].Alias = %q, want empty", source, i, imports[i].Alias)
		}
		if i > 0 && imports[i-1].Location.StartByte >= imports[i].Location.StartByte {
			t.Errorf("extractTSImports for %q: imports must be ordered by Location.StartByte ascending, got imports[%d].StartByte=%d >= imports[%d].StartByte=%d", source, i-1, imports[i-1].Location.StartByte, i, imports[i].Location.StartByte)
		}
	}
}

// AC-R3.2: a single-quoted module specifier must yield the bare path with
// no quotes and no error, proving strconv.Unquote is not used (it rejects
// single-quoted strings).
func TestExtractTSImports_SingleQuotedSpecifier(t *testing.T) {
	source := []byte(`import B from './b';`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	imports, err := extractTSImports(root, source)
	if err != nil {
		t.Fatalf("extractTSImports for single-quoted specifier %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractTSImports for single-quoted specifier %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	if imports[0].Path != "./b" {
		t.Errorf("extractTSImports for single-quoted specifier %q: Path = %q, want %q", source, imports[0].Path, "./b")
	}
}

// AC-R3.3: re-exports, require(...), and dynamic import(...) are all out
// of scope for v1 and must produce no ImportFeature.
func TestExtractTSImports_OutOfScopeFormsProduceNoImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "re-export", source: `export { X } from "./x";`},
		{name: "require", source: `const y = require("./y");`},
		{name: "dynamic import", source: `async function f() { await import("./z"); }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, closeTree := mustParseTS(t, []byte(tt.source))
			defer closeTree()

			imports, err := extractTSImports(root, []byte(tt.source))
			if err != nil {
				t.Fatalf("extractTSImports for %s %q: got err %v, want nil", tt.name, tt.source, err)
			}
			if len(imports) != 0 {
				t.Errorf("extractTSImports for %s %q: got %d imports (%+v), want 0 (out of scope for v1)", tt.name, tt.source, len(imports), imports)
			}
		})
	}
}

// AC-R3.4: a file with no imports yields a nil/empty Imports slice.
func TestExtractTSImports_NoImportsYieldsEmptySlice(t *testing.T) {
	source := []byte(`const x = 1;`)
	root, closeTree := mustParseTS(t, source)
	defer closeTree()

	imports, err := extractTSImports(root, source)
	if err != nil {
		t.Fatalf("extractTSImports for %q: got err %v, want nil", source, err)
	}
	if len(imports) != 0 {
		t.Errorf("extractTSImports for %q: got %d imports (%+v), want 0", source, len(imports), imports)
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

	imports, err := extractTSXImports(root, source)
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
