package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
