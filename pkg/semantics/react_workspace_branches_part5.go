package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// reactTabpanelBranch reports whether n is a JSX opening/self-closing
// element carrying role="tabpanel", and if so returns its branch: label is
// its aria-label attribute's string value, else its id attribute's string
// value, else the literal "tabpanel".
func reactTabpanelBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	attrs := reactJSXAttributes(n)
	if !reactHasStringJSXAttr(attrs, source, "role", "tabpanel") {
		return ReactWorkspaceBranch{}, false
	}
	label := reactFirstStringJSXAttr(attrs, source, "aria-label")
	if label == "" {
		label = reactFirstStringJSXAttr(attrs, source, "id")
	}
	if label == "" {
		label = "tabpanel"
	}
	return ReactWorkspaceBranch{Label: label, Location: locationFromNode(n)}, true
}
func reactHasStringJSXAttr(attrs []engine.Node, source []byte, name, want string) bool {
	for _, attr := range attrs {
		if reactJSXAttributeNameText(attr, source) != name {
			continue
		}
		if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok && s == want {
			return true
		}
	}
	return false
}

// reactCollectDiscriminantChain walks n forward through same-base chained
// ternary/if-else-if branches and returns the JSX-bearing branches found
// plus every chain node visited (regardless of JSX-bearing outcome), so the
// caller can mark the whole chain consumed even when the gate fails it.
func reactCollectDiscriminantChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	switch n.Kind() {
	case "ternary_expression":
		return reactCollectTernaryChain(n, base, source)
	case "if_statement":
		return reactCollectIfChain(n, base, source)
	default:
		return nil, nil
	}
}
