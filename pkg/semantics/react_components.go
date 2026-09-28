package semantics

import (
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

func reactCollectExportedComponents(root engine.Node, source []byte, hasDirective bool, bindings map[string]engine.Node) []ReactComponentFacts {
	var out []ReactComponentFacts
	for _, exportStmt := range reactExportStatements(root) {
		out = append(out, reactFactsFromExport(exportStmt, source, hasDirective, bindings)...)
	}
	return reactDedupeComponentsBySpan(out)
}

func reactFactsFromExport(exportStmt engine.Node, source []byte, hasDirective bool, bindings map[string]engine.Node) []ReactComponentFacts {
	var out []ReactComponentFacts
	for _, cand := range reactExportedCandidates(exportStmt, source, bindings) {
		rec, ok := reactBuildComponentFacts(cand, hasDirective, source)
		if !ok {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func reactExportStatements(root engine.Node) []engine.Node {
	var out []engine.Node
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		child := root.Child(i)
		if child.Kind() == "export_statement" {
			out = append(out, child)
		}
	}
	return out
}

func reactDedupeComponentsBySpan(in []ReactComponentFacts) []ReactComponentFacts {
	seen := map[[2]uint]struct{}{}
	var out []ReactComponentFacts
	for _, rec := range in {
		span := [2]uint{rec.Location.StartByte, rec.Location.EndByte}
		if _, dup := seen[span]; dup {
			continue
		}
		seen[span] = struct{}{}
		out = append(out, rec)
	}
	return out
}

// reactCandidate is one resolved exported function-like construct: funcNode
// is the actual component body owner (post one-level memo/forwardRef
// unwrap) and name is its resolved (possibly still empty) name.
type reactCandidate struct {
	funcNode engine.Node
	name     string
}

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

func reactSingleCandidate(funcNode engine.Node, name string) []reactCandidate {
	if funcNode == nil {
		return nil
	}
	return []reactCandidate{{funcNode: funcNode, name: name}}
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

func reactFindExportClause(exportStmt engine.Node) engine.Node {
	count := exportStmt.ChildCount()
	for i := 0; i < count; i++ {
		if c := exportStmt.Child(i); c.Kind() == "export_clause" {
			return c
		}
	}
	return nil
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

// reactResolveFunctionLike resolves n to the function-like node it denotes:
// itself directly for function_declaration/function_expression/
// arrow_function, or -- applying the normative one-level unwrap -- the
// first argument of a call_expression whose callee is exactly memo,
// React.memo, forwardRef, or React.forwardRef, when that argument is
// itself a function/arrow. Any other shape (a call to some other function,
// a non-function value, a nested wrapper call) returns nil.
func reactResolveFunctionLike(n engine.Node, source []byte) engine.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "function_declaration", "function_expression", "arrow_function":
		return n
	case "call_expression":
		if !isReactWrapperCallee(n, source) {
			return nil
		}
		args := n.ChildByFieldName("arguments")
		if args == nil {
			return nil
		}
		first := reactFirstArgumentNode(args)
		if first == nil {
			return nil
		}
		if first.Kind() == "function_expression" || first.Kind() == "arrow_function" {
			return first
		}
		return nil
	default:
		return nil
	}
}

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

func reactFirstArgumentNode(argsNode engine.Node) engine.Node {
	count := argsNode.ChildCount()
	for i := 0; i < count; i++ {
		c := argsNode.Child(i)
		switch c.Kind() {
		case "(", ")", ",":
			continue
		default:
			return c
		}
	}
	return nil
}

func isPascalCaseName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
