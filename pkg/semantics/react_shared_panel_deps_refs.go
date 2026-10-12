package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

type reactPanelDepRef struct {
	tag string
	id  string
}

func reactPanelDepTagIndex(body engine.Node, source []byte, allow map[string]struct{}) map[string]map[string]struct{} {
	return reactBuildDepTags(reactCollectPanelDepRefs(body, source, allow))
}

func reactCollectPanelDepRefs(body engine.Node, source []byte, allow map[string]struct{}) []reactPanelDepRef {
	var refs []reactPanelDepRef
	reactWalkScope(body, source, func(n engine.Node) {
		refs = append(refs, reactPanelDepRefsFromNode(n, source, allow)...)
	})
	return refs
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

func reactAllowedPanelDepRefs(n engine.Node, source []byte, allow map[string]struct{}) (tag string, ids []string, ok bool) {
	tag, ok = reactPascalCaseJSXTag(n, source)
	if !ok {
		return "", nil, false
	}
	for _, idName := range reactIdentifierJSXAttrValues(n, source) {
		if _, allowed := allow[idName]; allowed {
			ids = append(ids, idName)
		}
	}
	if len(ids) == 0 {
		return "", nil, false
	}
	return tag, ids, true
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
