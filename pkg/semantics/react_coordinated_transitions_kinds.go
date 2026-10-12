package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactTransitionKindAndName classifies fn as "effect" (first argument of
// useEffect/useLayoutEffect), "handler" (bound to an on*/handle* name via a
// declarator, assignment, object property, or JSX on* attribute), or
// "callback" (none of the above). Name is the assigned identifier, JSX
// attribute name, function's own name field, or the "<anonymous>" sentinel
// when none of those exist — never the empty string.
func reactTransitionKindAndName(fn engine.Node, source []byte) (kind, name string) {
	parent := fn.Parent()
	if kind, name, ok := reactTransitionEffect(fn, parent, source); ok {
		return kind, name
	}
	if parent == nil {
		return "callback", reactTransitionNameOrAnonymous(fn, source)
	}
	if kind, name, ok := reactTransitionBoundKind(parent, source); ok {
		return kind, name
	}
	if name, ok := reactTransitionJSXHandlerName(parent, source); ok {
		return "handler", name
	}
	return "callback", reactTransitionNameOrAnonymous(fn, source)
}

// reactTransitionNameOrAnonymous returns fn's own syntactic name field when
// present, otherwise the epic's "<anonymous>" sentinel.
func reactTransitionNameOrAnonymous(fn engine.Node, source []byte) string {
	if own := reactFuncOwnName(fn, source); own != "" {
		return own
	}
	return "<anonymous>"
}
