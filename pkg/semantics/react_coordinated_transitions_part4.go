package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactLocalFunctions(body engine.Node, source []byte) []engine.Node {
	var funcs []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "function_declaration", "function_expression", "arrow_function":
			if reactIsNestedComponent(n, source) {
				return
			}
			funcs = append(funcs, n)
		}
	})
	return funcs
}

// reactTransitionNameOrAnonymous returns fn's own syntactic name field when
// present, otherwise the epic's "<anonymous>" sentinel.
func reactTransitionNameOrAnonymous(fn engine.Node, source []byte) string {
	if own := reactFuncOwnName(fn, source); own != "" {
		return own
	}
	return "<anonymous>"
}
