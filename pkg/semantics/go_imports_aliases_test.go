package semantics

import (
	"testing"
)

// AC-3.2: an aliased import (`import f "fmt"`) must carry the alias
// identifier in ImportFeature.Alias.
func TestExtractImports_AliasedImport(t *testing.T) {
	source := []byte("package main\n\nimport f \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractGoImports(languageRegistry[LanguageGo].engineLang, root, source)
	if err != nil {
		t.Fatalf("extractGoImports for aliased import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractGoImports for aliased import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractGoImports for aliased import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "f" {
		t.Errorf("extractGoImports for aliased import %q: Alias = %q, want %q", source, got.Alias, "f")
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractGoImports for aliased import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}

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
