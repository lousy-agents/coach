package semantics

import (
	"testing"
)

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

	imports, err := extractTSImports(languageRegistry[LanguageTypeScript].engineLang, root, source)
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

	imports, err := extractTSImports(languageRegistry[LanguageTypeScript].engineLang, root, source)
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
