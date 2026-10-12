package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// reactCandidatesFromExportClause handles `export { Name }` /
// `export { Name as Alias }` / `export { Name as default }`: each specifier
// resolves against a same-module top-level binding by its local name (never
// the alias) -- no cross-file resolution.
func reactCandidatesFromExportClause(clause engine.Node, source []byte, bindings map[string]engine.Node) []reactCandidate {
	var out []reactCandidate
	count := clause.ChildCount()
	for i := 0; i < count; i++ {
		spec := clause.Child(i)
		if spec.Kind() != "export_specifier" {
			continue
		}
		nameField := spec.ChildByFieldName("name")
		if nameField == nil {
			continue
		}
		localName := nameField.Utf8Text(source)
		target, ok := bindings[localName]
		if !ok {
			continue
		}
		funcNode := reactResolveFunctionLike(target, source)
		if funcNode == nil {
			continue
		}
		fname := reactFuncOwnName(funcNode, source)
		if fname == "" {
			fname = localName
		}
		out = append(out, reactCandidate{funcNode: funcNode, name: fname})
	}
	return out
}
