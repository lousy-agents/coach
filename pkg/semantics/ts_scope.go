package semantics

import (
	"fmt"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsParamScope is one function-like construct's Finding-name half
// ("<function_or_method_name>") plus the set of identifier bindings visible
// in that scope. A binding value of true means the identifier is a parameter
// eligible for mutates_input; false means the identifier is a local binding
// that shadows an outer parameter but is not itself reportable here.
type tsParamScope struct {
	ownerName string
	bindings  map[string]bool
}

// newTSParamScope builds decl's tsParamScope: its Finding-name half (own
// "name" field's text, or "anonymous@<start_byte>" if it has none) and its
// identifier-bound parameter set (tsIdentifierParams).
func newTSParamScope(decl engine.Node, source []byte) tsParamScope {
	return tsParamScope{
		ownerName: tsFunctionOwnerName(decl, source),
		bindings:  tsIdentifierParams(decl, source),
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

func appendTSLocalBindings(scopes []tsParamScope, names map[string]bool) []tsParamScope {
	if len(names) == 0 || len(scopes) == 0 {
		return scopes
	}
	bindings := make(map[string]bool, len(names))
	for name := range names {
		bindings[name] = false
	}
	return append(scopes, tsParamScope{bindings: bindings})
}

func tsCurrentFunctionParamNames(scopes []tsParamScope) map[string]bool {
	for i := len(scopes) - 1; i >= 0; i-- {
		if scopes[i].ownerName == "" {
			continue
		}
		return paramNamesInScope(scopes[i].bindings)
	}
	return nil
}

func paramNamesInScope(bindings map[string]bool) map[string]bool {
	names := map[string]bool{}
	for name, isParam := range bindings {
		if isParam {
			names[name] = true
		}
	}
	return names
}
