package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactCandidate is one resolved exported function-like construct: funcNode
// is the actual component body owner (post one-level memo/forwardRef
// unwrap) and name is its resolved (possibly still empty) name.
type reactCandidate struct {
	funcNode engine.Node
	name     string
}

// reactExportedCandidates resolves exportStmt (a top-level export_statement)
// into zero or more reactCandidate entries, per the closed export-shape
// list: `export function Name`/`export default function Name` (field
// "declaration"), `export const Name = ...` (field "declaration"),
// `export default <expr>` (field "value": identifier, call expression, or
// an anonymous function/arrow), and `export { Name[, Name as Alias] }`
// (an export_clause child).
func reactExportedCandidates(exportStmt engine.Node, source []byte, bindings map[string]engine.Node) []reactCandidate {
	if decl := exportStmt.ChildByFieldName("declaration"); decl != nil {
		switch decl.Kind() {
		case "function_declaration":
			return reactSingleCandidate(decl, reactFuncOwnName(decl, source))
		case "lexical_declaration", "variable_declaration":
			return reactCandidatesFromDeclarators(decl, source)
		}
		return nil
	}
	if val := exportStmt.ChildByFieldName("value"); val != nil {
		return reactCandidatesFromExportValue(val, source, bindings)
	}
	if clause := reactFindExportClause(exportStmt); clause != nil {
		return reactCandidatesFromExportClause(clause, source, bindings)
	}
	return nil
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

func reactSingleCandidate(funcNode engine.Node, name string) []reactCandidate {
	if funcNode == nil {
		return nil
	}
	return []reactCandidate{{funcNode: funcNode, name: name}}
}

// reactFuncOwnName returns n's own syntactic "name" field text, or "" when
// n has none (anonymous function_expression, or any arrow_function).
func reactFuncOwnName(n engine.Node, source []byte) string {
	if n == nil {
		return ""
	}
	if name := n.ChildByFieldName("name"); name != nil {
		return name.Utf8Text(source)
	}
	return ""
}
