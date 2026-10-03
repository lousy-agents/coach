package semantics

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
		kinds := kindSet{}
		kinds.collect(tree.RootNode())
		t.Fatalf("AC-R1.3: minimal valid TSX source %q should parse without errors, node kinds seen: %s", source, dumpKinds(kinds))
	}
}

// kindSet collects every node kind string it encounters. It stands in for
// go-tree-sitter's ToSexp() (not part of the engine seam gotreesitter's Node
// satisfies): the discovery-gate tests below only need to prove a given node
// kind string exists somewhere in the parsed tree, not reproduce ToSexp()'s
// exact textual format.
type kindSet map[string]bool

func (kinds kindSet) collect(n engine.Node) {
	if n == nil {
		return
	}
	kinds[n.Kind()] = true
	for i := 0; i < n.ChildCount(); i++ {
		kinds.collect(n.Child(i))
	}
}
