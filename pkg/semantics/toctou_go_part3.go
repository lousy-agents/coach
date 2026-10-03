package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// findGoToctouActCall searches n's subtree (including inside any nested
// statements/if -- this detector does no scope resolution, only syntactic
// call/argument-text matching) for a call_expression named in
// goToctouActCallNames on the "os" package whose first argument's source
// text equals pathText, returning the first one found or nil.
func findGoToctouActCall(n engine.Node, source []byte, pathText string) engine.Node {
	if n == nil {
		return nil
	}
	if goToctouActMatch(n, source, pathText) {
		return n
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if found := findGoToctouActCall(n.Child(i), source, pathText); found != nil {
			return found
		}
	}
	return nil
}

func goToctouActMatch(n engine.Node, source []byte, pathText string) bool {
	if n.Kind() != "call_expression" {
		return false
	}
	pkg, name, ok := goSelectorCallInfo(n, source)
	if !ok || pkg != "os" || !goToctouActCallNames[name] {
		return false
	}
	arg := goCallFirstArgument(n)
	return arg != nil && arg.Utf8Text(source) == pathText
}

// goExpressionListValues returns list's non-punctuation children in source
// order (filtering out "," and any other literal tokens), i.e. the actual
// expression nodes an expression_list holds.
func goExpressionListValues(list engine.Node) []engine.Node {
	var out []engine.Node
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		if child.Kind() == "," {
			continue
		}
		out = append(out, child)
	}
	return out
}
