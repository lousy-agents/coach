package semantics

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// collectTSFunctionScopedVarDeclarationNames handles the
// variable_declaration case of tsCollectFunctionScopedNodeNames: node's own
// `var`-bound declarator names, excluding any already in params.
func (s nameSet) collectFunctionScopedVarDeclarationNames(node engine.Node, source []byte, params map[string]bool) {
	varNames := map[string]bool{}
	count := node.ChildCount()
	for i := 0; i < count; i++ {
		collectTSVariableDeclaratorNames(node.Child(i), source, varNames)
	}
	for name := range varNames {
		if !params[name] {
			s[name] = true
		}
	}
}

// tsFormalParameterIdentifierName reports p's bound identifier name with ok
// == true only when p is a required_parameter or optional_parameter with no
// default "value" field (a default like `q = 1` is excluded, per D5, same
// as a destructured or rest parameter) whose "pattern" field is itself a
// plain, non-destructured identifier.
func tsFormalParameterIdentifierName(p engine.Node, source []byte) (string, bool) {
	if p.Kind() != "required_parameter" && p.Kind() != "optional_parameter" {
		return "", false
	}
	if p.ChildByFieldName("value") != nil {
		return "", false
	}
	pattern := p.ChildByFieldName("pattern")
	if pattern == nil || pattern.Kind() != "identifier" {
		return "", false
	}
	return pattern.Utf8Text(source), true
}
func (s nameSet) collectReboundTargetNames(n engine.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "identifier", "shorthand_property_identifier_pattern":
		s[n.Utf8Text(source)] = true
	case "pair_pattern":
		s.collectReboundTargetNames(n.ChildByFieldName("value"), source)
	case "assignment_pattern":
		s.collectReboundTargetNames(n.ChildByFieldName("left"), source)
	case "rest_pattern":
		s.collectReboundTargetNames(n.ChildByFieldName("argument"), source)
	case "object_pattern", "array_pattern", "parenthesized_expression":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			s.collectReboundTargetNames(n.Child(i), source)
		}
	}
}

// tsFunctionOwnerName resolves decl's own Finding-name half: the source
// text of its syntactic "name" field (function_declaration,
// function_expression, generator_function[_declaration], and
// method_definition all expose one when named) or, when decl has no name
// field at all -- always true for arrow_function, and true for an
// anonymous function_expression -- "anonymous@<start_byte>". Per the
// issue spec this deliberately does not borrow a name from an enclosing
// variable_declarator (`const f = () => {}` still counts as anonymous):
// only decl's own syntactic name field counts.
func tsFunctionOwnerName(decl engine.Node, source []byte) string {
	if nameNode := decl.ChildByFieldName("name"); nameNode != nil {
		return nameNode.Utf8Text(source)
	}
	return fmt.Sprintf("anonymous@%d", decl.StartByte())
}
