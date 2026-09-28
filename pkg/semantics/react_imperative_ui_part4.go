package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactPascalCaseJSXTag(n engine.Node, source []byte) (string, bool) {
	switch n.Kind() {
	case "jsx_opening_element", "jsx_self_closing_element":
	default:
		return "", false
	}
	nameNode := n.ChildByFieldName("name")
	if nameNode == nil {
		return "", false
	}
	tag := nameNode.Utf8Text(source)
	if !isPascalCaseName(tag) {
		return "", false
	}
	return tag, true
}
func reactIdentifierJSXAttrValues(n engine.Node, source []byte) []string {
	var names []string
	for _, attr := range reactJSXAttributes(n) {
		val := reactJSXAttributeValueNode(attr)
		if val == nil || val.Kind() != "jsx_expression" {
			continue
		}
		inner := reactJSXExpressionInner(val)
		if inner == nil || inner.Kind() != "identifier" {
			continue
		}
		names = append(names, inner.Utf8Text(source))
	}
	return names
}
func reactBuildDepTags(refs []reactPanelDepRef) map[string]map[string]struct{} {
	depTags := map[string]map[string]struct{}{}
	for _, ref := range refs {
		if depTags[ref.id] == nil {
			depTags[ref.id] = map[string]struct{}{}
		}
		depTags[ref.id][ref.tag] = struct{}{}
	}
	return depTags
}
func reactPanelDepRefsFromNode(n engine.Node, source []byte, allow map[string]struct{}) []reactPanelDepRef {
	tag, ids, ok := reactAllowedPanelDepRefs(n, source, allow)
	if !ok {
		return nil
	}
	out := make([]reactPanelDepRef, 0, len(ids))
	for _, idName := range ids {
		out = append(out, reactPanelDepRef{tag: tag, id: idName})
	}
	return out
}
