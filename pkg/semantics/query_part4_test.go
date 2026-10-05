package semantics

import (
	"testing"
)

// AC-3.2: a raw-string (backtick) import path must have its backticks
// stripped from Path, with no Alias.
func TestExtractImports_RawStringBacktickPath(t *testing.T) {
	source := []byte("package main\n\nimport `fmt`\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractGoImports(languageRegistry[LanguageGo].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractGoImports for raw-string import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractGoImports for raw-string import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractGoImports for raw-string import %q: Path = %q, want %q (backticks stripped)", source, got.Path, "fmt")
	}
	if got.Alias != "" {
		t.Errorf("extractGoImports for raw-string import %q: Alias = %q, want empty", source, got.Alias)
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractGoImports for raw-string import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}
