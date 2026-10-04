package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsIdentifierParams collects decl's plain-identifier-bound parameter
// names (D5). arrow_function has two mutually exclusive parameter shapes:
// a bare single identifier (`p => ...`, field "parameter") or a
// parenthesized formal_parameters list (field "parameters"); every other
// function-like kind and method_definition only ever have "parameters".
// Each formal_parameters child is filtered per-parameter by
// tsFormalParameterIdentifierName, whose doc comment is the source of
// truth for what counts as identifier-bound.
func tsIdentifierParams(decl engine.Node, source []byte) map[string]bool {
	if params, ok := arrowBareParams(decl, source); ok {
		return params
	}
	params := map[string]bool{}

	formal := decl.ChildByFieldName("parameters")
	if formal == nil {
		return params
	}
	count := formal.ChildCount()
	for i := 0; i < count; i++ {
		if name, ok := tsFormalParameterIdentifierName(formal.Child(i), source); ok {
			params[name] = true
		}
	}
	return params
}

func arrowBareParams(decl engine.Node, source []byte) (map[string]bool, bool) {
	if decl.Kind() != "arrow_function" {
		return nil, false
	}
	bare := decl.ChildByFieldName("parameter")
	if bare == nil {
		return nil, false
	}
	params := map[string]bool{}
	if bare.Kind() == "identifier" {
		params[bare.Utf8Text(source)] = true
	}
	return params, true
}

func tsBlockScopedBindingNames(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "lexical_declaration":
			for j := 0; j < child.ChildCount(); j++ {
				collectTSVariableDeclaratorNames(child.Child(j), source, names)
			}
		case "function_declaration", "generator_function_declaration", "class_declaration":
			if name := child.ChildByFieldName("name"); name != nil {
				names[name.Utf8Text(source)] = true
			}
		}
	}
	return names
}
