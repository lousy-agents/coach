package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

func reactCapitalizedJSXChildLabel(cons engine.Node, source []byte) string {
	el := reactPrimaryJSXElement(cons)
	if el == nil {
		return ""
	}
	name := reactJSXElementTagName(el, source)
	if name == "" || !isPascalCaseName(name) {
		return ""
	}
	return name
}

func reactPrimaryJSXElement(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element":
		return n
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if el := reactPrimaryJSXElement(n.Child(i)); el != nil {
			return el
		}
	}
	return nil
}
