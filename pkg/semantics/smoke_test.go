package semantics

import (
	"strings"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
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

// AC-R1.3: a minimal valid .ts source must parse to a non-nil tree with no
// syntax errors under the pinned tree-sitter-typescript grammar, proving
// the grammar is ABI-compatible with go-tree-sitter v0.25.0 rather than
// asserted blind.
func TestSmoke_TSParsesMinimalProgramWithNoErrors(t *testing.T) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())); err != nil {
		t.Fatalf("AC-R1.3: setting the TypeScript grammar language failed: %v", err)
	}

	source := []byte("const x: number = 1;\n")
	tree := parser.Parse(source, nil)
	if tree == nil {
		t.Fatalf("AC-R1.3: Parse returned a nil tree for minimal valid TS source %q", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		t.Fatalf("AC-R1.3: minimal valid TS source %q should parse without errors, root S-expression: %s", source, tree.RootNode().ToSexp())
	}
}

// AC-R1.3 (TSX variant): a minimal valid .tsx source (containing JSX) must
// parse to a non-nil tree with no syntax errors under the TSX grammar.
func TestSmoke_TSXParsesMinimalProgramWithNoErrors(t *testing.T) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTSX())); err != nil {
		t.Fatalf("AC-R1.3: setting the TSX grammar language failed: %v", err)
	}

	source := []byte("const el = <div>hi</div>;\n")
	tree := parser.Parse(source, nil)
	if tree == nil {
		t.Fatalf("AC-R1.3: Parse returned a nil tree for minimal valid TSX source %q", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		t.Fatalf("AC-R1.3: minimal valid TSX source %q should parse without errors, root S-expression: %s", source, tree.RootNode().ToSexp())
	}
}

// TestSmoke_TSGrammarExposesExpectedNodeKinds is the D2a/D2b discovery gate
// for TypeScript: it verifies the exact grammar node kind strings
// computeTSFeatures depends on (statement kinds, the full function-like
// set, method_definition, and statement_block for nesting) against the
// pinned tree-sitter-typescript grammar, rather than trusting the issue
// spec's node-kind list blind. If any expected kind is absent, the actual
// S-expression is reported instead of silently adjusting the expected name.
func TestSmoke_TSGrammarExposesExpectedNodeKinds(t *testing.T) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())); err != nil {
		t.Fatalf("D2a/D2b discovery: setting the TypeScript grammar language failed: %v", err)
	}

	source := []byte(`function f(x: number) {
	if (x > 0) {
	}
	for (let i = 0; i < x; i++) {
	}
	for (const v of [1]) {
	}
	switch (x) {
		case 1:
			break;
	}
}
const arrow = () => {};
function* gen() {}
const genExpr = function* () {};
const fnExpr = function () {};
class C {
	constructor() {}
	method() {}
}
`)
	tree := parser.Parse(source, nil)
	if tree == nil {
		t.Fatalf("D2a/D2b discovery: Parse returned a nil tree for fixture %q", source)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("D2a/D2b discovery: fixture must parse cleanly to trust node-kind names, root S-expression: %s", root.ToSexp())
	}

	sexp := root.ToSexp()
	wantKinds := []string{
		"if_statement",
		"for_statement",
		"for_in_statement",
		"switch_statement",
		"function_declaration",
		"function_expression",
		"arrow_function",
		"generator_function_declaration",
		"generator_function",
		"method_definition",
		"statement_block",
		"property_identifier",
	}
	for _, kind := range wantKinds {
		if !strings.Contains(sexp, kind) {
			t.Fatalf("D2a/D2b discovery: expected grammar node kind %q not found in root S-expression; STOP and report actual S-expression instead of adjusting the expected kind name.\nactual S-expression: %s", kind, sexp)
		}
	}
}

// TestSmoke_TSGrammarExposesExpectedTightCouplingNodeKinds is the D3
// discovery gate: it verifies assignment_expression, new_expression, and
// the new_expression's "constructor" field are present with the expected
// shape for a `this.x = new Y()` inside a constructor, against the pinned
// grammar.
func TestSmoke_TSGrammarExposesExpectedTightCouplingNodeKinds(t *testing.T) {
	parser := tree_sitter.NewParser()
	defer parser.Close()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_typescript.LanguageTypescript())); err != nil {
		t.Fatalf("D3 discovery: setting the TypeScript grammar language failed: %v", err)
	}

	source := []byte(`class C {
	constructor() {
		this.svc = new HttpClient("http://x");
	}
}
`)
	tree := parser.Parse(source, nil)
	if tree == nil {
		t.Fatalf("D3 discovery: Parse returned a nil tree for fixture %q", source)
	}
	defer tree.Close()

	root := tree.RootNode()
	if root.HasError() {
		t.Fatalf("D3 discovery: fixture must parse cleanly to trust node-kind names, root S-expression: %s", root.ToSexp())
	}

	sexp := root.ToSexp()
	for _, kind := range []string{"assignment_expression", "new_expression", "constructor:"} {
		if !strings.Contains(sexp, kind) {
			t.Fatalf("D3 discovery: expected grammar node kind/field %q not found in root S-expression; STOP and report actual S-expression instead of adjusting the expected name.\nactual S-expression: %s", kind, sexp)
		}
	}
}
