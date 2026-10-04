package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactBuildComponentFacts applies the remaining candidacy gates (name
// present and PascalCase, JSX body, client gate) to cand and, if it passes,
// extracts its useState bindings and coordination facts.
func reactBuildComponentFacts(cand reactCandidate, hasDirective bool, source []byte) (ReactComponentFacts, bool) {
	if cand.funcNode == nil || !isPascalCaseName(cand.name) {
		return ReactComponentFacts{}, false
	}
	body := cand.funcNode.ChildByFieldName("body")
	if body == nil {
		return ReactComponentFacts{}, false
	}
	if !reactScopeContainsJSX(body, source) {
		return ReactComponentFacts{}, false
	}

	clientKind := ""
	switch {
	case hasDirective:
		clientKind = "use_client_directive"
	case reactScopeInvokesHook(body, source):
		clientKind = "hooks_and_jsx"
	default:
		return ReactComponentFacts{}, false
	}

	useState := reactExtractUseState(body, source)

	return ReactComponentFacts{
		Name:                   cand.name,
		Location:               locationFromNode(cand.funcNode),
		ClientKind:             clientKind,
		UseState:               useState,
		CoordinatedTransitions: reactExtractCoordinatedTransitions(body, source, useState),
		WorkspaceBranches:      reactExtractWorkspaceBranches(body, source),
		ImperativeUI:           reactExtractImperativeUI(body, source),
		SharedPanelDeps:        reactExtractSharedPanelDeps(body, source, useState),
	}, true
}
func reactModuleBindingsFrom(n engine.Node, source []byte) map[string]engine.Node {
	switch n.Kind() {
	case "function_declaration":
		name := n.ChildByFieldName("name")
		if name == nil {
			return nil
		}
		return map[string]engine.Node{name.Utf8Text(source): n}
	case "lexical_declaration", "variable_declaration":
		return reactDeclaratorBindingMap(n, source)
	case "export_statement":
		decl := n.ChildByFieldName("declaration")
		if decl == nil {
			return nil
		}
		return reactModuleBindingsFrom(decl, source)
	default:
		return nil
	}
}

// reactWalkScope visits n and its descendants in pre-order, but does not
// descend into a nested non-exported PascalCase-named function/arrow's
// subtree: that inner component is excluded entirely from the outer
// candidate's fact walk. A nested non-PascalCase helper's subtree remains
// fully visited -- state/calls inside it attribute to the outer candidate.
func reactWalkScope(n engine.Node, source []byte, visit func(engine.Node)) {
	if n == nil {
		return
	}
	visit(n)
	if reactIsNestedComponent(n, source) {
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		reactWalkScope(n.Child(i), source, visit)
	}
}
