package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactResolveFunctionLike resolves n to the function-like node it denotes:
// itself directly for function_declaration/function_expression/
// arrow_function, or -- applying the normative one-level unwrap -- the
// first argument of a call_expression whose callee is exactly memo,
// React.memo, forwardRef, or React.forwardRef, when that argument is
// itself a function/arrow. Any other shape (a call to some other function,
// a non-function value, a nested wrapper call) returns nil.
func reactResolveFunctionLike(n engine.Node, source []byte) engine.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "function_declaration", "function_expression", "arrow_function":
		return n
	case "call_expression":
		if !isReactWrapperCallee(n, source) {
			return nil
		}
		args := n.ChildByFieldName("arguments")
		if args == nil {
			return nil
		}
		first := reactFirstArgumentNode(args)
		if first == nil {
			return nil
		}
		if first.Kind() == "function_expression" || first.Kind() == "arrow_function" {
			return first
		}
		return nil
	default:
		return nil
	}
}
func reactDeclaratorBindingMap(n engine.Node, source []byte) map[string]engine.Node {
	out := map[string]engine.Node{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		d := n.Child(i)
		if d.Kind() != "variable_declarator" {
			continue
		}
		name := d.ChildByFieldName("name")
		value := d.ChildByFieldName("value")
		if name == nil || name.Kind() != "identifier" || value == nil {
			continue
		}
		out[name.Utf8Text(source)] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
