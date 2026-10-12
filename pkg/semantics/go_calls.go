package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// goSelectorCallInfo reports call's package and function name if call is a
// call_expression whose callee is a selector_expression on a bare
// identifier operand (`pkg.Name(...)`), e.g. ("os", "Stat", true) for
// `os.Stat(path)`. It reports ok == false for any other callee shape (a
// bare identifier callee, a chained/method selector, a call result, etc.)
// or if call is not a call_expression.
func goSelectorCallInfo(call engine.Node, source []byte) (pkg, name string, ok bool) {
	if call == nil || call.Kind() != "call_expression" {
		return "", "", false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "selector_expression" {
		return "", "", false
	}
	operand := fn.ChildByFieldName("operand")
	field := fn.ChildByFieldName("field")
	if operand == nil || operand.Kind() != "identifier" || field == nil {
		return "", "", false
	}
	return operand.Utf8Text(source), field.Utf8Text(source), true
}

// goCallFirstArgument returns call's "arguments" field's first non-
// punctuation child (the first argument expression), or nil if call has no
// arguments.
func goCallFirstArgument(call engine.Node) engine.Node {
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
