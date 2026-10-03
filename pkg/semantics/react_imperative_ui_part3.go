package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
