package semantics

import (
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

func TestSmoke_ParsesMinimalProgramWithNoErrors(t *testing.T) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language())); err != nil {
		t.Fatalf("toolchain proof: setting the Go grammar language failed: %v", err)
	}

	source := []byte("package main\nfunc main() {}\n")
	tree := parser.Parse(source, nil)
	if tree == nil {
		t.Fatalf("toolchain proof: Parse returned a nil tree for minimal valid source %q", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		t.Fatalf("toolchain proof: minimal valid source %q should parse without errors, root S-expression: %s", source, tree.RootNode().ToSexp())
	}
}

// TestSmoke_GrammarExposesExpectedStatementNodeKinds is the AC-3.3 discovery
// gate: it verifies the exact grammar node kind strings the spec's
// structural-metrics traversal (Task 5) depends on. Per the issue's stop
// conditions, if any expected kind is absent here, implementation must stop
// and report the actual S-expression rather than adjusting the expected
// names to match reality.
func TestSmoke_GrammarExposesExpectedStatementNodeKinds(t *testing.T) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_go.Language())); err != nil {
		t.Fatalf("AC-3.3 discovery: setting the Go grammar language failed: %v", err)
	}

	source := []byte(`package main

func f(x int) {
	if x > 0 {
	}
	for i := 0; i < x; i++ {
	}
	switch x {
	case 1:
	}
	switch v := any(x).(type) {
	case int:
		_ = v
	}
	select {
	default:
	}
}
`)
	tree := parser.Parse(source, nil)
	if tree == nil {
		t.Fatalf("AC-3.3 discovery: Parse returned a nil tree for fixture %q", source)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("AC-3.3 discovery: fixture must parse cleanly to trust node-kind names, root S-expression: %s", root.ToSexp())
	}

	sexp := root.ToSexp()
	wantKinds := []string{
		"if_statement",
		"for_statement",
		"expression_switch_statement",
		"type_switch_statement",
		"select_statement",
	}
	for _, kind := range wantKinds {
		if !strings.Contains(sexp, kind) {
			t.Fatalf("AC-3.3 discovery: expected grammar node kind %q not found in root S-expression; STOP per issue constraint #3 and report actual S-expression instead of adjusting the expected kind name.\nactual S-expression: %s", kind, sexp)
		}
	}
}
