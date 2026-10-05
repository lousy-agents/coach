package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// goToctouStatCallNames is the CWE-367 "check" call name set (Story 3):
// os.Stat and os.Lstat both return (FileInfo, error) and both observe a
// path's current state that can change before an "act" call on the same
// path runs.
var goToctouStatCallNames = map[string]bool{
	"Stat":  true,
	"Lstat": true,
}

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

// isGoErrNilGate reports whether cond is a direct nil-comparison
// binary_expression on the identifier named errName -- `err == nil` or `nil
// == err`, either operand order -- the only valid success gate for v1. Any
// other condition shape (`err != nil`, a sentinel check like
// errors.Is(err, fs.ErrNotExist) or os.IsNotExist(err), a boolean
// combination, etc.) reports false.
func isGoErrNilGate(cond engine.Node, errName string, source []byte) bool {
	if cond == nil || errName == "" || cond.Kind() != "binary_expression" {
		return false
	}
	if goBinaryOp(cond, source) != "==" {
		return false
	}
	left := cond.ChildByFieldName("left")
	right := cond.ChildByFieldName("right")
	if left == nil || right == nil {
		return false
	}
	if left.Kind() == "nil" && right.Kind() == "identifier" && right.Utf8Text(source) == errName {
		return true
	}
	if right.Kind() == "nil" && left.Kind() == "identifier" && left.Utf8Text(source) == errName {
		return true
	}
	return false
}
