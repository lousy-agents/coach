package semantics

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
