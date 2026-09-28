package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// findTSToctouActCall searches n's subtree (including inside any nested
// function-like constructs -- this detector does no scope resolution, only
// syntactic call/argument-text matching) for a call_expression named in
// tsToctouActCallNames whose first argument's source text equals pathText,
// returning the first one found or nil.
func findTSToctouActCall(n engine.Node, source []byte, pathText string) engine.Node {
	if n == nil {
		return nil
	}
	if tsToctouActMatch(n, source, pathText) {
		return n
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if found := findTSToctouActCall(n.Child(i), source, pathText); found != nil {
			return found
		}
	}
	return nil
}

func tsToctouActMatch(n engine.Node, source []byte, pathText string) bool {
	if n.Kind() != "call_expression" {
		return false
	}
	name, ok := tsCallFunctionName(n, source)
	if !ok || !tsToctouActCallNames[name] {
		return false
	}
	arg := tsCallFirstArgument(n)
	return arg != nil && arg.Utf8Text(source) == pathText
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

// tsToctouCallArg returns call's first argument node if call is a
// call_expression named wantName -- a bare identifier callee (`wantName(...)`)
// or a member_expression callee whose "property" field's text is wantName
// (`<obj>.wantName(...)`), for any object -- or nil otherwise.
func tsToctouCallArg(call engine.Node, source []byte, wantName string) engine.Node {
	name, ok := tsCallFunctionName(call, source)
	if !ok || name != wantName {
		return nil
	}
	return tsCallFirstArgument(call)
}
