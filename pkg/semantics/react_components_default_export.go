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
		// Arrow functions never carry their own name field, and a direct
		// `export default (...) => ...` has no outer binding to fall back
		// to, so this is always the anonymous ("") case (candidacy fails).
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
