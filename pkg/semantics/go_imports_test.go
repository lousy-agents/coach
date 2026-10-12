package semantics

import (
	"testing"
)

// AC-3.1: a grouped import declaration must yield one ImportFeature per
// spec, ordered by Location.StartByte ascending (AC-1.10), which here
// matches source declaration order.
func TestExtractImports_GroupedImports(t *testing.T) {
	source := []byte("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractGoImports(languageRegistry[LanguageGo].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractGoImports for grouped imports %q: got err %v, want nil", source, err)
	}

	if len(imports) != 2 {
		t.Fatalf("extractGoImports for grouped imports %q: got %d imports (%+v), want exactly 2", source, len(imports), imports)
	}
	if imports[0].Path != "fmt" {
		t.Errorf("extractGoImports for grouped imports %q: imports[0].Path = %q, want %q (byte-order first)", source, imports[0].Path, "fmt")
	}
	if imports[1].Path != "os" {
		t.Errorf("extractGoImports for grouped imports %q: imports[1].Path = %q, want %q (byte-order second)", source, imports[1].Path, "os")
	}
	if imports[0].Location.StartByte >= imports[1].Location.StartByte {
		t.Errorf("extractGoImports for grouped imports %q: imports[0].Location.StartByte=%d, imports[1].Location.StartByte=%d, want strictly ascending order (AC-1.10)", source, imports[0].Location.StartByte, imports[1].Location.StartByte)
	}
	for _, imp := range imports {
		if imp.Alias != "" {
			t.Errorf("extractGoImports for grouped imports %q: Alias = %q for path %q, want empty", source, imp.Alias, imp.Path)
		}
	}
}

// Regression guard raised by review: stripping only the leading/trailing
// delimiter leaves escape sequences (e.g. \x2f) in Path as raw source text
// instead of interpreting them the way the Go compiler would. Path must be
// the interpreted string value.
func TestExtractImports_InterpretedStringPathWithEscapeSequence(t *testing.T) {
	source := []byte(`package main

import "example.com/foo\x2fbar"
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractGoImports(languageRegistry[LanguageGo].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractGoImports for escaped import path %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractGoImports for escaped import path %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	want := "example.com/foo/bar"
	if got.Path != want {
		t.Errorf("extractGoImports for escaped import path %q: Path = %q, want %q (interpreted, not raw escape text)", source, got.Path, want)
	}
}
