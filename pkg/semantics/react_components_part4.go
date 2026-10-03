package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactCandidatesFromExportValue handles `export default <expr>`: val is
// exportStmt's "value" field, present exactly when the exported expression
// is not itself a named declaration (anonymous function/arrow, a bare
// identifier referencing a same-module binding, or a call expression such
// as memo(...)/forwardRef(...)).
func reactCandidatesFromExportValue(val engine.Node, source []byte, bindings map[string]engine.Node) []reactCandidate {
	switch val.Kind() {
	case "function_expression":
		return reactSingleCandidate(val, reactFuncOwnName(val, source))
	case "arrow_function":

		return reactSingleCandidate(val, "")
	case "identifier":
		target, ok := bindings[val.Utf8Text(source)]
		if !ok {
			return nil
		}
		funcNode := reactResolveFunctionLike(target, source)
		if funcNode == nil {
			return nil
		}
		fname := reactFuncOwnName(funcNode, source)
		if fname == "" {
			fname = val.Utf8Text(source)
		}
		return reactSingleCandidate(funcNode, fname)
	case "call_expression":
		funcNode := reactResolveFunctionLike(val, source)
		if funcNode == nil {
			return nil
		}
		return reactSingleCandidate(funcNode, reactFuncOwnName(funcNode, source))
	default:
		return nil
	}
}

// reactCandidatesFromDeclarators handles `export const/let/var Name = ...`:
// each plain-identifier-bound declarator whose value resolves to a
// function-like construct (directly, or through one memo/forwardRef
// unwrap) becomes a candidate.
func reactCandidatesFromDeclarators(declNode engine.Node, source []byte) []reactCandidate {
	var out []reactCandidate
	count := declNode.ChildCount()
	for i := 0; i < count; i++ {
		d := declNode.Child(i)
		if d.Kind() != "variable_declarator" {
			continue
		}
		name := d.ChildByFieldName("name")
		value := d.ChildByFieldName("value")
		if name == nil || name.Kind() != "identifier" || value == nil {
			continue
		}
		funcNode := reactResolveFunctionLike(value, source)
		if funcNode == nil {
			continue
		}
		fname := reactFuncOwnName(funcNode, source)
		if fname == "" {
			fname = name.Utf8Text(source)
		}
		out = append(out, reactCandidate{funcNode: funcNode, name: fname})
	}
	return out
}
