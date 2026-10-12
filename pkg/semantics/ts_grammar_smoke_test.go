package semantics

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// TestSmoke_TSGrammarExposesExpectedNodeKinds is the D2a/D2b discovery gate
// for TypeScript: it verifies the exact grammar node kind strings
// computeTSFeatures depends on (statement kinds, the full function-like
// set, method_definition, and statement_block for nesting) against the
// pinned TypeScript grammar, rather than trusting the issue spec's
// node-kind list blind. If any expected kind is absent, the actual node
// kinds are reported instead of silently adjusting the expected name.
func TestSmoke_TSGrammarExposesExpectedNodeKinds(t *testing.T) {
	lang := engine.GoTreeSitterLanguage("typescript")
	parser, err := lang.NewParser()
	if err != nil {
		t.Fatalf("D2a/D2b discovery: creating the TypeScript grammar parser failed: %v", err)
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
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("D2a/D2b discovery: Parse returned an error for fixture %q: %v", source, err)
	}
	if tree == nil {
		t.Fatalf("D2a/D2b discovery: Parse returned a nil tree for fixture %q", source)
	}
	defer tree.Close()

	root := tree.RootNode()
	kinds := kindSet{}
	kinds.collect(root)
	if root.HasError() {
		t.Fatalf("D2a/D2b discovery: fixture must parse cleanly to trust node-kind names, node kinds seen: %s", dumpKinds(kinds))
	}

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
		if !kinds[kind] {
			t.Fatalf("D2a/D2b discovery: expected grammar node kind %q not found; STOP and report actual node kinds instead of adjusting the expected kind name.\nnode kinds seen: %s", kind, dumpKinds(kinds))
		}
	}
}

// AC-R1.3: a minimal valid .ts source must parse to a non-nil tree with no
// syntax errors under the pinned TypeScript grammar, proving the grammar is
// wired up correctly rather than asserted blind.
func TestSmoke_TSParsesMinimalProgramWithNoErrors(t *testing.T) {
	lang := engine.GoTreeSitterLanguage("typescript")
	parser, err := lang.NewParser()
	if err != nil {
		t.Fatalf("AC-R1.3: creating the TypeScript grammar parser failed: %v", err)
	}

	source := []byte("const x: number = 1;\n")
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("AC-R1.3: Parse returned an error for minimal valid TS source %q: %v", source, err)
	}
	if tree == nil {
		t.Fatalf("AC-R1.3: Parse returned a nil tree for minimal valid TS source %q", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		kinds := kindSet{}
		kinds.collect(tree.RootNode())
		t.Fatalf("AC-R1.3: minimal valid TS source %q should parse without errors, node kinds seen: %s", source, dumpKinds(kinds))
	}
}
