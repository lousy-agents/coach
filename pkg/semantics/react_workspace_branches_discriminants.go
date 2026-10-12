package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactDiscriminantOps is the set of equality operators a workspace-branch
// discriminant condition may use.
var reactDiscriminantOps = map[string]bool{"===": true, "!==": true, "==": true, "!=": true}

// reactDiscriminantConditionBase reports whether n's "condition" field is a
// binary_expression testing equality/inequality between a discriminant base
// (a plain identifier, or a member_expression -- whose full text, not just
// its property, is used as the base so structurally distinct discriminants
// like props.v and other.v never compare equal) and a string/number
// literal, and returns that base text.
func reactDiscriminantConditionBase(n engine.Node, source []byte) (string, bool) {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return "", false
	}
	if !reactDiscriminantOps[tsBinaryOp(cond)] {
		return "", false
	}
	left := cond.ChildByFieldName("left")
	right := cond.ChildByFieldName("right")
	if left == nil || right == nil {
		return "", false
	}
	if base, ok := reactDiscriminantBase(left, right, source); ok {
		return base, true
	}
	return reactDiscriminantBase(right, left, source)
}

func reactDiscriminantBase(baseSide, literalSide engine.Node, source []byte) (string, bool) {
	if literalSide == nil || (literalSide.Kind() != "string" && literalSide.Kind() != "number") {
		return "", false
	}
	switch baseSide.Kind() {
	case "identifier":
		return baseSide.Utf8Text(source), true
	case "member_expression":
		return baseSide.Utf8Text(source), true
	}
	return "", false
}

func reactDiscriminantLiteralLabel(n engine.Node, source []byte) string {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return ""
	}
	if lbl, ok := reactLiteralLabelText(cond.ChildByFieldName("left"), source); ok {
		return lbl
	}
	if lbl, ok := reactLiteralLabelText(cond.ChildByFieldName("right"), source); ok {
		return lbl
	}
	return ""
}

func reactLiteralLabelText(n engine.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case "string":
		return reactBareStringText(n, source)
	case "number":
		return n.Utf8Text(source), true
	default:
		return "", false
	}
}
