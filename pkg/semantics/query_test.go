package semantics

import (
	"context"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

// mustParseGo parses source as Go and returns its root node plus a cleanup
// function that closes the underlying tree. It reuses the syntaxParser seam
// already exercised by parser_test.go rather than duplicating Tree-sitter
// setup here.
func mustParseGo(t *testing.T, source []byte) (*tree_sitter.Node, func()) {
	t.Helper()

	sp := newSyntaxParser()
	tree, err := sp.parse(context.Background(), source)
	if err != nil {
		t.Fatalf("parsing Go source %q: got err %v, want nil", source, err)
	}
	return tree.RootNode(), tree.Close
}

// Smoke check (not itself an AC): the import-extraction query must compile
// against the real Go grammar before any capture-matching logic is layered
// on top, so a bad query surfaces as its own failure rather than being
// buried inside a fixture test.
func TestImportQuerySource_CompilesAgainstGoGrammar(t *testing.T) {
	lang := tree_sitter.NewLanguage(tree_sitter_go.Language())

	query, queryErr := tree_sitter.NewQuery(lang, importQuerySource)
	if queryErr != nil {
		t.Fatalf("compiling import query %q against the Go grammar: got err %v, want nil", importQuerySource, queryErr)
	}
	defer query.Close()
}

// AC-3.1: a single import declaration must yield exactly one ImportFeature
// with its quotes stripped from Path and no Alias.
func TestExtractImports_SingleImport(t *testing.T) {
	source := []byte("package main\n\nimport \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractImports(root, source)
	if err != nil {
		t.Fatalf("extractImports for single import %q: got err %v, want nil", source, err)
	}

	if len(imports) != 1 {
		t.Fatalf("extractImports for single import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractImports for single import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "" {
		t.Errorf("extractImports for single import %q: Alias = %q, want empty", source, got.Alias)
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractImports for single import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}

// AC-3.1: a grouped import declaration must yield one ImportFeature per
// spec, ordered by Location.StartByte ascending (AC-1.10), which here
// matches source declaration order.
func TestExtractImports_GroupedImports(t *testing.T) {
	source := []byte("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractImports(root, source)
	if err != nil {
		t.Fatalf("extractImports for grouped imports %q: got err %v, want nil", source, err)
	}

	if len(imports) != 2 {
		t.Fatalf("extractImports for grouped imports %q: got %d imports (%+v), want exactly 2", source, len(imports), imports)
	}
	if imports[0].Path != "fmt" {
		t.Errorf("extractImports for grouped imports %q: imports[0].Path = %q, want %q (byte-order first)", source, imports[0].Path, "fmt")
	}
	if imports[1].Path != "os" {
		t.Errorf("extractImports for grouped imports %q: imports[1].Path = %q, want %q (byte-order second)", source, imports[1].Path, "os")
	}
	if imports[0].Location.StartByte >= imports[1].Location.StartByte {
		t.Errorf("extractImports for grouped imports %q: imports[0].Location.StartByte=%d, imports[1].Location.StartByte=%d, want strictly ascending order (AC-1.10)", source, imports[0].Location.StartByte, imports[1].Location.StartByte)
	}
	for _, imp := range imports {
		if imp.Alias != "" {
			t.Errorf("extractImports for grouped imports %q: Alias = %q for path %q, want empty", source, imp.Alias, imp.Path)
		}
	}
}

// AC-3.2: an aliased import (`import f "fmt"`) must carry the alias
// identifier in ImportFeature.Alias.
func TestExtractImports_AliasedImport(t *testing.T) {
	source := []byte("package main\n\nimport f \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractImports(root, source)
	if err != nil {
		t.Fatalf("extractImports for aliased import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractImports for aliased import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractImports for aliased import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "f" {
		t.Errorf("extractImports for aliased import %q: Alias = %q, want %q", source, got.Alias, "f")
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractImports for aliased import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}

// AC-3.2: a dot import (`import . "fmt"`) must carry "." in
// ImportFeature.Alias.
func TestExtractImports_DotImport(t *testing.T) {
	source := []byte("package main\n\nimport . \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractImports(root, source)
	if err != nil {
		t.Fatalf("extractImports for dot import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractImports for dot import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractImports for dot import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "." {
		t.Errorf("extractImports for dot import %q: Alias = %q, want %q", source, got.Alias, ".")
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractImports for dot import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}

// AC-3.2: a blank import (`import _ "fmt"`) must carry "_" in
// ImportFeature.Alias.
func TestExtractImports_BlankImport(t *testing.T) {
	source := []byte("package main\n\nimport _ \"fmt\"\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractImports(root, source)
	if err != nil {
		t.Fatalf("extractImports for blank import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractImports for blank import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractImports for blank import %q: Path = %q, want %q", source, got.Path, "fmt")
	}
	if got.Alias != "_" {
		t.Errorf("extractImports for blank import %q: Alias = %q, want %q", source, got.Alias, "_")
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractImports for blank import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}

// AC-3.2: a raw-string (backtick) import path must have its backticks
// stripped from Path, with no Alias.
func TestExtractImports_RawStringBacktickPath(t *testing.T) {
	source := []byte("package main\n\nimport `fmt`\n")
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	imports, err := extractImports(root, source)
	if err != nil {
		t.Fatalf("extractImports for raw-string import %q: got err %v, want nil", source, err)
	}
	if len(imports) != 1 {
		t.Fatalf("extractImports for raw-string import %q: got %d imports (%+v), want exactly 1", source, len(imports), imports)
	}
	got := imports[0]
	if got.Path != "fmt" {
		t.Errorf("extractImports for raw-string import %q: Path = %q, want %q (backticks stripped)", source, got.Path, "fmt")
	}
	if got.Alias != "" {
		t.Errorf("extractImports for raw-string import %q: Alias = %q, want empty", source, got.Alias)
	}
	if got.Location.StartByte >= got.Location.EndByte {
		t.Errorf("extractImports for raw-string import %q: Location = %+v, want a non-zero-width span", source, got.Location)
	}
}
