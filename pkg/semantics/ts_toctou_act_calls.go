package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsToctouActCallNames is the CWE-367 "act" file-operation call name set
// (Story 1): synchronous Node fs calls whose target path can change between
// a preceding existsSync gate observing it and this call acting on it.
var tsToctouActCallNames = map[string]bool{
	"readFileSync":   true,
	"writeFileSync":  true,
	"appendFileSync": true,
	"unlinkSync":     true,
	"rmSync":         true,
}

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
