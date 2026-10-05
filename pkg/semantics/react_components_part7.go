package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"

	"sort"
)

// isReactWrapperCallee reports whether call's callee is exactly one of the
// four supported HOC wrappers: memo, React.memo, forwardRef, or
// React.forwardRef.
func isReactWrapperCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		name := fn.Utf8Text(source)
		return name == "memo" || name == "forwardRef"
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		name := prop.Utf8Text(source)
		return name == "memo" || name == "forwardRef"
	default:
		return false
	}
}

// reactBareStringText strips n's quote characters (single or double),
// mirroring reactIsUseClientLiteral's quoting rule (strconv.Unquote
// rejects JS single-quoted strings, so it is not reused here).
func reactBareStringText(n engine.Node, source []byte) (string, bool) {
	if n == nil || n.Kind() != "string" {
		return "", false
	}
	text := n.Utf8Text(source)
	if len(text) < 2 {
		return "", false
	}
	quote := text[0]
	if (quote != '"' && quote != '\'') || text[len(text)-1] != quote {
		return "", false
	}
	return text[1 : len(text)-1], true
}

// computeReactComponents discovers every candidate React component among
// root's top-level exports (TS/TSX only) and extracts its useState
// bindings plus its coordination facts (CoordinatedTransitions,
// WorkspaceBranches, ImperativeUI, SharedPanelDeps).
func computeReactComponents(root engine.Node, source []byte) []ReactComponentFacts {
	if root == nil {
		return nil
	}

	hasDirective := moduleHasUseClientDirective(root, source)
	bindings := collectModuleTopLevelBindings(root, source)
	out := reactCollectExportedComponents(root, source, hasDirective, bindings)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Location.StartByte != out[j].Location.StartByte {
			return out[i].Location.StartByte < out[j].Location.StartByte
		}
		return out[i].Name < out[j].Name
	})
	return out
}
func reactFindExportClause(exportStmt engine.Node) engine.Node {
	count := exportStmt.ChildCount()
	for i := 0; i < count; i++ {
		if c := exportStmt.Child(i); c.Kind() == "export_clause" {
			return c
		}
	}
	return nil
}
