package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactExtractCoordinatedTransitions finds every function/arrow body in
// body's scan set (reactWalkScope) that calls at least two distinct
// useState setters, and returns one ReactCoordinatedTransition per such
// body, ordered by the body's own start_byte. Calls to the same setter
// within one body count once; setters not present in useState (never
// resolved, e.g. destructure fell through to "") never match.
func reactExtractCoordinatedTransitions(body engine.Node, source []byte, useState []ReactUseStateBinding) []ReactCoordinatedTransition {
	setters := reactUseStateSetterIndex(useState)
	if len(setters) == 0 {
		return nil
	}

	var out []ReactCoordinatedTransition
	for _, fn := range reactLocalFunctions(body, source) {
		updated := reactBindingsUpdatedBy(fn, source, setters)
		if len(updated) < 2 {
			continue
		}
		kind, name := reactTransitionKindAndName(fn, source)
		out = append(out, ReactCoordinatedTransition{
			Name:            name,
			Kind:            kind,
			Location:        locationFromNode(fn),
			UpdatedBindings: updated,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Location.StartByte < out[j].Location.StartByte
	})
	return out
}

func reactUseStateSetterIndex(useState []ReactUseStateBinding) map[string]string {
	setters := map[string]string{}
	for _, u := range useState {
		if u.Setter != "" && u.Binding != "" {
			setters[u.Setter] = u.Binding
		}
	}
	return setters
}

func reactLocalFunctions(body engine.Node, source []byte) []engine.Node {
	var funcs []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "function_declaration", "function_expression", "arrow_function":
			if reactIsNestedComponent(n, source) {
				return
			}
			funcs = append(funcs, n)
		}
	})
	return funcs
}
