package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactImperativeAPINames is the closed set of imperative DOM/UI APIs
// recorded; any other callee/property name is ignored.
var reactImperativeAPINames = map[string]bool{
	"getElementById":   true,
	"querySelector":    true,
	"querySelectorAll": true,
	"focus":            true,
	"blur":             true,
	"scrollIntoView":   true,
}

// reactExtractImperativeUI collects one ReactImperativeUICall per
// call_expression in body's scan set whose resolved API name (the callee
// identifier, or a member_expression callee's property -- covering both
// `.` and `?.` uniformly, since optional chaining has no distinct node
// kind) is in the closed reactImperativeAPINames set, ordered by the call's
// own start_byte.
func reactExtractImperativeUI(body engine.Node, source []byte) []ReactImperativeUICall {
	var calls []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		if reactImperativeAPI(n, source) != "" {
			calls = append(calls, n)
		}
	})
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].StartByte() < calls[j].StartByte()
	})

	out := make([]ReactImperativeUICall, 0, len(calls))
	for _, c := range calls {
		out = append(out, ReactImperativeUICall{
			API:      reactImperativeAPI(c, source),
			Location: locationFromNode(c),
		})
	}
	return out
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

func reactSharedDepsFromTags(depTags map[string]map[string]struct{}) []ReactSharedPanelDep {
	var names []string
	for name, tags := range depTags {
		if len(tags) >= 2 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	out := make([]ReactSharedPanelDep, 0, len(names))
	for _, name := range names {
		tags := depTags[name]
		panels := make([]string, 0, len(tags))
		for t := range tags {
			panels = append(panels, t)
		}
		sort.Strings(panels)
		out = append(out, ReactSharedPanelDep{Name: name, Panels: panels})
	}
	return out
}

// reactKnownSharedDepNames builds the allowlist for shared panel deps: every
// non-empty useState binding name, plus every local function/arrow binding
// name in body's scan set (known callbacks/handlers). Nested PascalCase
// component names are excluded. Setters and non-function module values are
// not included.
func reactKnownSharedDepNames(body engine.Node, source []byte, useState []ReactUseStateBinding) map[string]struct{} {
	known := make(map[string]struct{}, len(useState))
	for _, u := range useState {
		if u.Binding != "" {
			known[u.Binding] = struct{}{}
		}
	}
	reactWalkScope(body, source, func(n engine.Node) {
		if name, ok := reactLocalFunctionBindingName(n, source); ok {
			known[name] = struct{}{}
		}
	})
	return known
}

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

func reactBoundIdentifierName(parent engine.Node, source []byte) (string, bool) {
	if parent == nil {
		return "", false
	}
	switch parent.Kind() {
	case "variable_declarator":
		nameNode := parent.ChildByFieldName("name")
		if nameNode == nil || nameNode.Kind() != "identifier" {
			return "", false
		}
		nme := nameNode.Utf8Text(source)
		if nme == "" {
			return "", false
		}
		return nme, true
	case "assignment_expression":
		left := parent.ChildByFieldName("left")
		if left == nil || left.Kind() != "identifier" {
			return "", false
		}
		nme := left.Utf8Text(source)
		if nme == "" {
			return "", false
		}
		return nme, true
	default:
		return "", false
	}
}
