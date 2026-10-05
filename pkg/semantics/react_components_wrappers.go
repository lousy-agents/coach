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

// isReactWrapperCallee reports whether call's callee is exactly one of the
// four supported HOC wrappers: memo, React.memo, forwardRef, or
// React.forwardRef.
func isReactWrapperCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		name := fn.Utf8Text(source)
		return name == "memo" || name == "forwardRef"
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		name := prop.Utf8Text(source)
		return name == "memo" || name == "forwardRef"
	default:
		return false
	}
}

func reactFirstArgumentNode(argsNode engine.Node) engine.Node {
	count := argsNode.ChildCount()
	for i := 0; i < count; i++ {
		c := argsNode.Child(i)
		switch c.Kind() {
		case "(", ")", ",":
			continue
		default:
			return c
		}
	}
	return nil
}
