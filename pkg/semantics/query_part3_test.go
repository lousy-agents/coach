package semantics

import (
	"testing"
)

// AC-3.2: a dot import (`import . "fmt"`) must carry "." in
// ImportFeature.Alias.
func TestExtractImports_DotImport(t *testing.T) {
	source := []byte("package main\n\nimport . \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractGoImports(languageRegistry[LanguageGo].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractGoImports for dot import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractGoImports for dot import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractGoImports for dot import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "." {
		t.Errorf("extractGoImports for dot import %q: Alias = %q, want %q", source, got.Alias, ".")
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractGoImports for dot import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}

// AC-3.2: a blank import (`import _ "fmt"`) must carry "_" in
// ImportFeature.Alias.
func TestExtractImports_BlankImport(t *testing.T) {
	source := []byte("package main\n\nimport _ \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractGoImports(languageRegistry[LanguageGo].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractGoImports for blank import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractGoImports for blank import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractGoImports for blank import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "_" {
		t.Errorf("extractGoImports for blank import %q: Alias = %q, want %q", source, got.Alias, "_")
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractGoImports for blank import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}
