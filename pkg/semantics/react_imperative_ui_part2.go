package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactLocalFunctionBindingName(n engine.Node, source []byte) (string, bool) {
	switch n.Kind() {
	case "function_declaration":
		if reactIsNestedComponent(n, source) {
			return "", false
		}
		name := n.ChildByFieldName("name")
		if name == nil {
			return "", false
		}
		nme := name.Utf8Text(source)
		if nme == "" {
			return "", false
		}
		return nme, true
	case "function_expression", "arrow_function":
		if reactIsNestedComponent(n, source) {
			return "", false
		}
		return reactBoundIdentifierName(n.Parent(), source)
	default:
		return "", false
	}
}
func reactImperativeAPI(call engine.Node, source []byte) string {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return ""
	}
	var api string
	switch fn.Kind() {
	case "member_expression":
		prop := fn.ChildByFieldName("property")
		if prop == nil {
			return ""
		}
		api = prop.Utf8Text(source)
	case "identifier":
		api = fn.Utf8Text(source)
	default:
		return ""
	}
	if reactImperativeAPINames[api] {
		return api
	}
	return ""
}

// reactExtractSharedPanelDeps groups every `attr={identifier}` JSX
// attribute (identifier value, not a member expression/spread/literal)
// found on an uppercase-tag JSX element in body's scan set by identifier
// name, and returns one ReactSharedPanelDep per identifier that is a known
// state binding or in-component callback/handler name and is referenced this
// way by >=2 distinct tag names, ordered by Name. Module-level constants and
// other non-allowlisted identifiers never qualify (Supporting C isolation).
func reactExtractSharedPanelDeps(body engine.Node, source []byte, useState []ReactUseStateBinding) []ReactSharedPanelDep {
	allow := reactKnownSharedDepNames(body, source, useState)
	if len(allow) == 0 {
		return nil
	}
	return reactSharedDepsFromTags(reactPanelDepTagIndex(body, source, allow))
}
