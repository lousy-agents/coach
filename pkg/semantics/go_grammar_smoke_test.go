package semantics

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
	kinds := kindSet{}
	kinds.collect(root)
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
		kinds := kindSet{}
		kinds.collect(tree.RootNode())
		t.Fatalf("toolchain proof: minimal valid source %q should parse without errors, node kinds seen: %s", source, dumpKinds(kinds))
	}
}
