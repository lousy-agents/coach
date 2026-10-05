package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// goStatInitializerCall extracts an os.Stat/os.Lstat call from ifStmt's own
// "initializer" field and reports the identifier bound to its second
// (error) result value -- Stat/Lstat's signature is (FileInfo, error), so
// the second bound identifier is always the error result regardless of its
// name. Go's grammar uses "short_var_declaration" for `:=` and
// "assignment_statement" for `=`; both share the same "left"/"right" field
// shapes, so both are handled identically here. It reports call == nil for
// any other initializer shape, a left side that doesn't bind exactly two
// values, or a right side that isn't exactly one os.Stat/os.Lstat
// call_expression.
func goStatInitializerCall(ifStmt engine.Node, source []byte) (call engine.Node, errName string) {
	init := ifStmt.ChildByFieldName("initializer")
	if init == nil {
		return nil, ""
	}
	switch init.Kind() {
	case "short_var_declaration", "assignment_statement":
	default:
		return nil, ""
	}

	left := init.ChildByFieldName("left")
	right := init.ChildByFieldName("right")
	if left == nil || right == nil {
		return nil, ""
	}

	targets := goExpressionListValues(left)
	if len(targets) != 2 || targets[1].Kind() != "identifier" {
		return nil, ""
	}

	values := goExpressionListValues(right)
	if len(values) != 1 {
		return nil, ""
	}
	rhs := values[0]
	pkg, name, ok := goSelectorCallInfo(rhs, source)
	if !ok || pkg != "os" || !goToctouStatCallNames[name] {
		return nil, ""
	}

	return rhs, targets[1].Utf8Text(source)
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
