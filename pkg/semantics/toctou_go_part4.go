package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
