package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// shadowNames returns a copy of outer with any entries whose name appears in
// names removed.
func shadowNames(outer map[string]paramMutKind, names map[string]bool) map[string]paramMutKind {
	if len(outer) == 0 || len(names) == 0 {
		return outer
	}

	shadowed := make(map[string]paramMutKind, len(outer))
	for name, kind := range outer {
		if names[name] {
			continue
		}
		shadowed[name] = kind
	}
	return shadowed
}

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

// checkConstructorFunc emits a "constructor_func" Finding (AC-3.5) if decl's
// name field matches constructorFuncNameRe.
func (c *featureCollector) checkConstructorFunc(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(source)
	if !constructorFuncNameRe.MatchString(name) {
		return
	}
	c.findings = append(c.findings, Finding{
		Kind:     "constructor_func",
		Name:     name,
		Location: locationFromNode(decl),
	})
}

// checkMutatesInput emits one "mutates_input" Finding per distinct
// (parameter, mutation-expression location) pair where decl's body writes
// through a syntactically pointer/map/slice-typed parameter, either via a
// selector on the parameter or a dereference of it (cfg.Name = x,
// (*cfg).Name = x) or via index assignment on a map/slice parameter
// (values[k] = x, items[i] = x). Plain reassignment of the parameter
// variable itself (cfg = other) is a rebind, not a caller-visible mutation,
// and is deliberately excluded.
func (c *featureCollector) checkMutatesInput(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	params := decl.ChildByFieldName("parameters")
	body := decl.ChildByFieldName("body")
	if nameNode == nil || params == nil || body == nil {
		return
	}
	funcName := nameNode.Utf8Text(source)
	mutableParams := mutableParamTypes(params, source)
	if len(mutableParams) == 0 {
		return
	}

	c.mutatesSeen = map[mutatesInputKey]bool{}
	c.findAssignments(body, source, funcName, mutableParams)
}
