package semantics

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// collectKinds walks n and all its descendants, collecting every node kind
// string it encounters. It stands in for go-tree-sitter's ToSexp() (not part
// of the engine seam gotreesitter's Node satisfies): the discovery-gate
// tests below only need to prove a given node kind string exists somewhere
// in the parsed tree, not reproduce ToSexp()'s exact textual format.
func collectKinds(n engine.Node, kinds map[string]bool) {
	if n == nil {
		return
	}
	kinds[n.Kind()] = true
	for i := 0; i < n.ChildCount(); i++ {
		collectKinds(n.Child(i), kinds)
	}
}

func dumpKinds(kinds map[string]bool) string {
	all := make([]string, 0, len(kinds))
	for k := range kinds {
		all = append(all, k)
	}
	return strings.Join(all, ", ")
}

func TestSmoke_ParsesMinimalProgramWithNoErrors(t *testing.T) {
	lang := engine.GoTreeSitterLanguage("go")
	parser, err := lang.NewParser()
	if err != nil {
		t.Fatalf("toolchain proof: creating the Go grammar parser failed: %v", err)
	}

	source := []byte("package main\nfunc main() {}\n")
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("toolchain proof: Parse returned an error for minimal valid source %q: %v", source, err)
	}
	if tree == nil {
		t.Fatalf("toolchain proof: Parse returned a nil tree for minimal valid source %q", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		kinds := map[string]bool{}
		collectKinds(tree.RootNode(), kinds)
		t.Fatalf("toolchain proof: minimal valid source %q should parse without errors, node kinds seen: %s", source, dumpKinds(kinds))
	}
}

// TestSmoke_GrammarExposesExpectedStatementNodeKinds is the AC-3.3 discovery
// gate: it verifies the exact grammar node kind strings the spec's
// structural-metrics traversal (Task 5) depends on. Per the issue's stop
// conditions, if any expected kind is absent here, implementation must stop
// and report the actual node kinds rather than adjusting the expected
// names to match reality.
func TestSmoke_GrammarExposesExpectedStatementNodeKinds(t *testing.T) {
	lang := engine.GoTreeSitterLanguage("go")
	parser, err := lang.NewParser()
	if err != nil {
		t.Fatalf("AC-3.3 discovery: creating the Go grammar parser failed: %v", err)
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
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("AC-3.3 discovery: Parse returned an error for fixture %q: %v", source, err)
	}
	if tree == nil {
		t.Fatalf("AC-3.3 discovery: Parse returned a nil tree for fixture %q", source)
	}
	defer tree.Close()

	root := tree.RootNode()
	kinds := map[string]bool{}
	collectKinds(root, kinds)
	if root.HasError() {
		t.Fatalf("AC-3.3 discovery: fixture must parse cleanly to trust node-kind names, node kinds seen: %s", dumpKinds(kinds))
	}

	wantKinds := []string{
		"if_statement",
		"for_statement",
		"expression_switch_statement",
		"type_switch_statement",
		"select_statement",
	}
	for _, kind := range wantKinds {
		if !kinds[kind] {
			t.Fatalf("AC-3.3 discovery: expected grammar node kind %q not found; STOP per issue constraint #3 and report actual node kinds instead of adjusting the expected kind name.\nnode kinds seen: %s", kind, dumpKinds(kinds))
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
		kinds := map[string]bool{}
		collectKinds(tree.RootNode(), kinds)
		t.Fatalf("AC-R1.3: minimal valid TS source %q should parse without errors, node kinds seen: %s", source, dumpKinds(kinds))
	}
}

// AC-R1.3 (TSX variant): a minimal valid .tsx source (containing JSX) must
// parse to a non-nil tree with no syntax errors under the TSX grammar.
func TestSmoke_TSXParsesMinimalProgramWithNoErrors(t *testing.T) {
	lang := engine.GoTreeSitterLanguage("tsx")
	parser, err := lang.NewParser()
	if err != nil {
		t.Fatalf("AC-R1.3: creating the TSX grammar parser failed: %v", err)
	}

	source := []byte("const el = <div>hi</div>;\n")
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("AC-R1.3: Parse returned an error for minimal valid TSX source %q: %v", source, err)
	}
	if tree == nil {
		t.Fatalf("AC-R1.3: Parse returned a nil tree for minimal valid TSX source %q", source)
	}
	defer tree.Close()

	if tree.RootNode().HasError() {
		kinds := map[string]bool{}
		collectKinds(tree.RootNode(), kinds)
		t.Fatalf("AC-R1.3: minimal valid TSX source %q should parse without errors, node kinds seen: %s", source, dumpKinds(kinds))
	}
}

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
	kinds := map[string]bool{}
	collectKinds(root, kinds)
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

// TestSmoke_TSGrammarExposesExpectedTightCouplingNodeKinds is the D3
// discovery gate: it verifies assignment_expression, new_expression, and
// the new_expression's "constructor" field are present with the expected
// shape for a `this.x = new Y()` inside a constructor, against the pinned
// grammar.
func TestSmoke_TSGrammarExposesExpectedTightCouplingNodeKinds(t *testing.T) {
	lang := engine.GoTreeSitterLanguage("typescript")
	parser, err := lang.NewParser()
	if err != nil {
		t.Fatalf("D3 discovery: creating the TypeScript grammar parser failed: %v", err)
	}

	source := []byte(`class C {
	constructor() {
		this.svc = new HttpClient("http://x");
	}
}
`)
	tree, err := parser.Parse(source)
	if err != nil {
		t.Fatalf("D3 discovery: Parse returned an error for fixture %q: %v", source, err)
	}
	if tree == nil {
		t.Fatalf("D3 discovery: Parse returned a nil tree for fixture %q", source)
	}
	defer tree.Close()

	root := tree.RootNode()
	kinds := map[string]bool{}
	collectKinds(root, kinds)
	if root.HasError() {
		t.Fatalf("D3 discovery: fixture must parse cleanly to trust node-kind names, node kinds seen: %s", dumpKinds(kinds))
	}

	var hasConstructorField bool
	var walkFields func(n engine.Node)
	walkFields = func(n engine.Node) {
		if n == nil {
			return
		}
		if n.Kind() == "new_expression" && n.ChildByFieldName("constructor") != nil {
			hasConstructorField = true
		}
		for i := 0; i < n.ChildCount(); i++ {
			walkFields(n.Child(i))
		}
	}
	walkFields(root)

	for _, kind := range []string{"assignment_expression", "new_expression"} {
		if !kinds[kind] {
			t.Fatalf("D3 discovery: expected grammar node kind %q not found; STOP and report actual node kinds instead of adjusting the expected name.\nnode kinds seen: %s", kind, dumpKinds(kinds))
		}
	}
	if !hasConstructorField {
		t.Fatalf("D3 discovery: expected new_expression to expose a \"constructor\" field; STOP and report actual node kinds instead of adjusting the expected name.\nnode kinds seen: %s", dumpKinds(kinds))
	}
}
