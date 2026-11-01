package semantics

import (
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

// tsAnonymousFunctionOwnerName names a function with no syntactic name. It
// carries no source position because the name is part of a signal's
// identity: a byte offset would change whenever an edit above the function
// shifts it. tsFunctionOwnerName prefixes it with the nearest function name
// or the name the function is held under (tsAssignedName: a plain variable,
// assignment target property, class field, or object property), so anonymous
// functions under differently named holders get different names, while those
// under the same name share one.
const tsAnonymousFunctionOwnerName = "<func lit>"

// newTSParamScope builds decl's tsParamScope: its Finding-name half (own
// "name" field's text, or tsAnonymousFunctionOwnerName if it has none) and
// its identifier-bound parameter set (tsIdentifierParams).
func newTSParamScope(decl engine.Node, source []byte) tsParamScope {
	bindings := tsIdentifierParams(decl, source)
	if accumulator, ok := tsFreshReduceAccumulatorName(decl, source); ok {
		bindings[accumulator] = false
	}
	return tsParamScope{
		ownerName: tsFunctionOwnerName(decl, source),
		bindings:  bindings,
	}
}

// tsFunctionOwnerName resolves decl's own Finding-name half: the source
// text of its syntactic "name" field (function_declaration,
// function_expression, generator_function[_declaration], and
// method_definition all expose one when named) or, when decl has no name
// field at all -- always true for arrow_function, and true for an
// anonymous function_expression -- "<context>/<func lit>", where <context>
// is tsNearestFunctionName, or just tsAnonymousFunctionOwnerName when no
// function or variable name is reachable. A function bound to a variable
// (`const f = () => {}`) is still anonymous: the variable name is only a
// prefix of its "<func lit>" name, never the function's own name.
func tsFunctionOwnerName(decl engine.Node, source []byte) string {
	if nameNode := decl.ChildByFieldName("name"); nameNode != nil {
		return nameNode.Utf8Text(source)
	}
	if context := tsNearestFunctionName(decl, source); context != "" {
		return context + "/" + tsAnonymousFunctionOwnerName
	}
	return tsAnonymousFunctionOwnerName
}

// tsNearestFunctionName returns the name of the nearest function-like node,
// starting at decl itself and walking outward, that has one: its own
// syntactic name, or else the name it is held under (tsAssignedName).
// Anonymous functions that are held under no name are skipped, so a callback
// nested in other callbacks takes the closest name. It returns "" when none
// is reachable, as for an immediately invoked function or module-level code.
func tsNearestFunctionName(decl engine.Node, source []byte) string {
	for node := decl; node != nil; node = node.Parent() {
		if !tsFunctionLikeKinds[node.Kind()] && node.Kind() != "method_definition" {
			continue
		}
		if nameNode := node.ChildByFieldName("name"); nameNode != nil {
			if node.Kind() == "method_definition" {
				return tsQualifiedMethodName(node, nameNode, source)
			}
			return nameNode.Utf8Text(source)
		}
		if assigned := tsAssignedName(node, source); assigned != "" {
			return assigned
		}
	}
	return ""
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
