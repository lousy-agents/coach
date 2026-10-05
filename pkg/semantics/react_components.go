package semantics

import (
	"regexp"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactHookNamePattern matches a hook-shaped identifier/property name: an
// uppercase letter immediately after the "use" prefix (e.g. useState,
// useEffect), per the React hooks naming convention this package relies on
// for the no-directive client gate.
var reactHookNamePattern = regexp.MustCompile(`^use[A-Z]`)

// computeReactComponents discovers every candidate React component among
// root's top-level exports (TS/TSX only) and extracts its useState
// bindings plus its coordination facts (CoordinatedTransitions,
// WorkspaceBranches, ImperativeUI, SharedPanelDeps).

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

// reactCandidatesFromDeclarators handles `export const/let/var Name = ...`:
// each plain-identifier-bound declarator whose value resolves to a
// function-like construct (directly, or through one memo/forwardRef
// unwrap) becomes a candidate.

// reactCandidatesFromExportValue handles `export default <expr>`: val is
// exportStmt's "value" field, present exactly when the exported expression
// is not itself a named declaration (anonymous function/arrow, a bare
// identifier referencing a same-module binding, or a call expression such
// as memo(...)/forwardRef(...)).

// Arrow functions never carry their own name field, and a direct
// `export default (...) => ...` has no outer binding to fall back
// to, so this is always the anonymous ("") case (candidacy fails).

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

// reactFuncOwnName returns n's own syntactic "name" field text, or "" when
// n has none (anonymous function_expression, or any arrow_function).

// reactResolveFunctionLike resolves n to the function-like node it denotes:
// itself directly for function_declaration/function_expression/
// arrow_function, or -- applying the normative one-level unwrap -- the
// first argument of a call_expression whose callee is exactly memo,
// React.memo, forwardRef, or React.forwardRef, when that argument is
// itself a function/arrow. Any other shape (a call to some other function,
// a non-function value, a nested wrapper call) returns nil.

// isReactWrapperCallee reports whether call's callee is exactly one of the
// four supported HOC wrappers: memo, React.memo, forwardRef, or
// React.forwardRef.

// collectModuleTopLevelBindings maps every top-level function declaration
// and plain-identifier-bound const/let/var name to its declaration/
// initializer node, for resolving `export default Name` and
// `export { Name }` against a same-module binding. A top-level statement
// wrapped in `export ...` is also registered under its own name (e.g.
// `export const C = ...` registers "C"). A binding registered this way can
// still be referenced by another export form in the same module (e.g.
// `export const Page = ...` plus `export default Page;`); computeReactComponents
// dedupes the resulting candidates by resolved function node span so such
// re-exports produce one record, not two.

// moduleHasUseClientDirective reports whether root's first non-comment
// top-level statement is an expression_statement whose sole expression is
// the string literal "use client" (either quote style).

// reactIsUseClientLiteral reports whether str's raw source text is exactly
// "use client" or 'use client'. tsStringLiteralText is not reused here
// because it delegates to strconv.Unquote, which rejects JS single-quoted
// string literals (a Go-only quoting rule), silently dropping the
// single-quoted spelling of the directive.

// reactWalkScope visits n and its descendants in pre-order, but does not
// descend into a nested non-exported PascalCase-named function/arrow's
// subtree: that inner component is excluded entirely from the outer
// candidate's fact walk. A nested non-PascalCase helper's subtree remains
// fully visited -- state/calls inside it attribute to the outer candidate.

// reactExtractUseState collects every direct useState()/React.useState()
// call within body's scope (per reactWalkScope's nesting rules), ordered by
// call start_byte ascending.

// isReactUseStateCallee reports whether call's callee is exactly useState
// or React.useState. Aliased imports (`import { useState as us }`) are a
// deliberately accepted false negative -- this package does not resolve
// import aliases.

// reactUseStateBindingNames resolves call's binding/setter pair from its
// enclosing variable_declarator, when call is exactly that declarator's own
// initializer: `const [x, setX] = useState(...)` -> ("x", "setX") when the
// destructure's 2nd element is a plain identifier, else ("x", "");
// `const x = useState(...)` -> ("x", ""); anything else (call not assigned
// via a declarator, or an unsupported name pattern) -> ("", "").

// reactArrayPatternElements returns n's elements by position, including a
// nil placeholder for an elided element (e.g. `[, setC]`), so that index 0
// is always the binding slot and index 1 the setter slot even when earlier
// elements are holes.

// reactJSXAttributeValueNode returns attr's value node -- attr's child 2,
// either a bare string or a jsx_expression -- or nil when attr has no
// value (e.g. a boolean shorthand attribute).

// reactJSXExpressionInner returns the expression node wrapped by a
// jsx_expression ("{" expr "}"), skipping the brace tokens, or nil when
// none is present.

// reactBareStringText strips n's quote characters (single or double),
// mirroring reactIsUseClientLiteral's quoting rule (strconv.Unquote
// rejects JS single-quoted strings, so it is not reused here).
