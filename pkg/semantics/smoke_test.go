package semantics

import (
	"sort"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
	kinds := kindSet{}
	kinds.collect(root)
	if root.HasError() {
		t.Fatalf("D3 discovery: fixture must parse cleanly to trust node-kind names, node kinds seen: %s", dumpKinds(kinds))
	}

	var hasConstructorField bool
	var walkFields func(n engine.Node)
	walkFields = (&sigTestSmokeTSGrammarExposesExpectedTightCouplingNodeKinds21{hasConstructorField: &hasConstructorField, walkFields: walkFields}).call

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

func dumpKinds(kinds kindSet) string {
	all := make([]string, 0, len(kinds))
	for k := range kinds {
		all = append(all, k)
	}
	sort.Strings(all)
	return strings.Join(all, ", ")
}
