package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// checkPointerReturn emits a "pointer_return" Finding (AC-3.6) if decl's
// result field contains a pointer_type, either directly (a single unnamed
// pointer return value) or among a parameter_list's parameter_declaration
// types (multiple and/or named return values).
func (c *featureCollector) checkPointerReturn(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	result := decl.ChildByFieldName("result")
	if result == nil || !resultHasPointerType(result) {
		return
	}
	c.findings = append(c.findings, Finding{
		Kind:     "pointer_return",
		Name:     nameNode.Utf8Text(source),
		Location: locationFromNode(decl),
	})
}

// resultHasPointerType reports whether a function/method's result field
// contains a pointer_type anywhere in its subtree: as the result node
// itself (a single unnamed pointer return value), as a parameter_list's
// parameter_declaration type (multiple and/or named return values), or
// nested inside a composite type such as a slice, map value, or channel
// element (e.g. []*T, map[string]*T, chan *T). A full descendant search is
// used rather than checking only the direct result/type node, since Go
// permits pointer_type at any depth within a composite result type.
func resultHasPointerType(result engine.Node) bool {
	if result.Kind() == "pointer_type" {
		return true
	}
	count := result.ChildCount()
	for i := 0; i < count; i++ {
		if resultHasPointerType(result.Child(i)) {
			return true
		}
	}
	return false
}
