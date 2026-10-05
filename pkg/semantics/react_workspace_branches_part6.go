package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// reactExtractWorkspaceBranches collects ReactWorkspaceBranch entries from
// two disjoint constructs: discriminant ternary/if-else-if chains gated at
// >=3 JSX-bearing branches per chain, and role="tabpanel" JSX elements
// (ungated). Results are de-duplicated by the branch's own Location.StartByte
// and ordered by that same start_byte.
func reactExtractWorkspaceBranches(body engine.Node, source []byte) []ReactWorkspaceBranch {
	var branches []ReactWorkspaceBranch
	seen := map[uint]struct{}{}
	addBranch := func(b ReactWorkspaceBranch) {
		if _, dup := seen[b.Location.StartByte]; dup {
			return
		}
		seen[b.Location.StartByte] = struct{}{}
		branches = append(branches, b)
	}

	reactCollectDiscriminantBranches(body, source, addBranch)
	reactCollectTabpanelBranches(body, source, addBranch)

	sort.SliceStable(branches, func(i, j int) bool {
		return branches[i].Location.StartByte < branches[j].Location.StartByte
	})
	return branches
}
