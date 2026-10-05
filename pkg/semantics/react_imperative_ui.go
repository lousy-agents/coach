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

// reactExtractSharedPanelDeps groups every `attr={identifier}` JSX
// attribute (identifier value, not a member expression/spread/literal)
// found on an uppercase-tag JSX element in body's scan set by identifier
// name, and returns one ReactSharedPanelDep per identifier that is a known
// state binding or in-component callback/handler name and is referenced this
// way by >=2 distinct tag names, ordered by Name. Module-level constants and
// other non-allowlisted identifiers never qualify (Supporting C isolation).

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
