package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactBindingsUpdatedBy(fn engine.Node, source []byte, setters map[string]string) []string {
	updated := map[string]struct{}{}
	reactWalkLocalScope(fn.ChildByFieldName("body"), func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		callee := n.ChildByFieldName("function")
		if callee == nil || callee.Kind() != "identifier" {
			return
		}
		if binding, ok := setters[callee.Utf8Text(source)]; ok {
			updated[binding] = struct{}{}
		}
	})
	if len(updated) == 0 {
		return nil
	}
	bindings := make([]string, 0, len(updated))
	for b := range updated {
		bindings = append(bindings, b)
	}
	sort.Strings(bindings)
	return bindings
}

// reactWalkLocalScope visits n and its descendants in pre-order but does
// not descend past a nested function/arrow boundary -- unlike
// reactWalkScope, it has no PascalCase-component exception, since any
// function boundary already stops it. Used to attribute calls to the
// function body that directly contains them, not an enclosing one.
func reactWalkLocalScope(n engine.Node, visit func(engine.Node)) {
	if n == nil {
		return
	}
	visit(n)
	switch n.Kind() {
	case "function_declaration", "function_expression", "arrow_function":
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		reactWalkLocalScope(n.Child(i), visit)
	}
}
