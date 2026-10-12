package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsCallFunctionName resolves call's callee name: the bare identifier
// (`existsSync`) or the "property" field's text for a member_expression
// callee (`fs.existsSync`), regardless of the object. It reports ok == false
// for any other callee shape (e.g. a call result: `f()()`) or if call is
// not a call_expression.
func tsCallFunctionName(call engine.Node, source []byte) (name string, ok bool) {
	if call == nil || call.Kind() != "call_expression" {
		return "", false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return "", false
	}
	switch fn.Kind() {
	case "identifier":
		return fn.Utf8Text(source), true
	case "member_expression":
		property := fn.ChildByFieldName("property")
		if property == nil {
			return "", false
		}
		return property.Utf8Text(source), true
	default:
		return "", false
	}
}

// tsCallFirstArgument returns call's "arguments" field's first non-
// punctuation child (the first argument expression), or nil if call has no
// arguments.
func tsCallFirstArgument(call engine.Node) engine.Node {
	args := call.ChildByFieldName("arguments")
	if args == nil {
		return nil
	}
	count := args.ChildCount()
	for i := 0; i < count; i++ {
		child := args.Child(i)
		switch child.Kind() {
		case "(", ")", ",":
			continue
		}
		return child
	}
	return nil
}
