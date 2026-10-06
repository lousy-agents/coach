package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactCollectTabpanelBranches(body engine.Node, source []byte, add func(ReactWorkspaceBranch)) {
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "jsx_opening_element", "jsx_self_closing_element":
		default:
			return
		}
		if b, ok := reactTabpanelBranch(n, source); ok {
			add(b)
		}
	})
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

func reactFirstStringJSXAttr(attrs []engine.Node, source []byte, name string) string {
	for _, attr := range attrs {
		if reactJSXAttributeNameText(attr, source) != name {
			continue
		}
		if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok {
			return s
		}
	}
	return ""
}
