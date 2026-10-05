package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactContainsJSX(n engine.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
		return true
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if reactContainsJSX(n.Child(i)) {
			return true
		}
	}
	return false
}

// reactIfChainContinue returns the next if_statement in an else-if chain, or
// a terminal residual branch when the chain ends in a final else body.
func reactIfChainContinue(cur engine.Node, source []byte) (next engine.Node, terminal *ReactWorkspaceBranch) {
	alt := cur.ChildByFieldName("alternative")
	if alt != nil && alt.Kind() == "else_clause" {
		alt = tsElseClauseInner(alt)
	}
	if alt != nil && alt.Kind() == "if_statement" {
		return alt, nil
	}
	if b, ok := reactResidualBranch(alt, source); ok {
		return nil, &b
	}
	return nil, nil
}

// reactWorkspaceBranchLabel applies the label precedence rule: the
// discriminant condition's literal text first, else a capitalized primary
// JSX child's tag name, else the "<branch>" sentinel.
func reactWorkspaceBranchLabel(condHolder, cons engine.Node, source []byte) string {
	if condHolder != nil {
		if lbl := reactDiscriminantLiteralLabel(condHolder, source); lbl != "" {
			return lbl
		}
	}
	if lbl := reactCapitalizedJSXChildLabel(cons, source); lbl != "" {
		return lbl
	}
	return "<branch>"
}
