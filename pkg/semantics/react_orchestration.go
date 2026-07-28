package semantics

import (
	"regexp"
	"sort"
	"unicode"
	"unicode/utf8"

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
func computeReactComponents(root engine.Node, source []byte) []ReactComponentFacts {
	if root == nil {
		return nil
	}

	hasDirective := moduleHasUseClientDirective(root, source)
	bindings := collectModuleTopLevelBindings(root, source)

	var out []ReactComponentFacts
	seen := map[[2]uint]struct{}{}
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		child := root.Child(i)
		if child.Kind() != "export_statement" {
			continue
		}
		for _, cand := range reactExportedCandidates(child, source, bindings) {
			span := [2]uint{cand.funcNode.StartByte(), cand.funcNode.EndByte()}
			if _, dup := seen[span]; dup {
				continue
			}
			if rec, ok := reactBuildComponentFacts(cand, hasDirective, source); ok {
				seen[span] = struct{}{}
				out = append(out, rec)
			}
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Location.StartByte != out[j].Location.StartByte {
			return out[i].Location.StartByte < out[j].Location.StartByte
		}
		return out[i].Name < out[j].Name
	})
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
		SharedPanelDeps:        reactExtractSharedPanelDeps(body, source),
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
func collectModuleTopLevelBindings(root engine.Node, source []byte) map[string]engine.Node {
	bindings := map[string]engine.Node{}
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		reactCollectTopLevelBinding(root.Child(i), source, bindings)
	}
	return bindings
}

func reactCollectTopLevelBinding(n engine.Node, source []byte, bindings map[string]engine.Node) {
	switch n.Kind() {
	case "function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			bindings[name.Utf8Text(source)] = n
		}
	case "lexical_declaration", "variable_declaration":
		reactCollectDeclaratorBindings(n, source, bindings)
	case "export_statement":
		if decl := n.ChildByFieldName("declaration"); decl != nil {
			reactCollectTopLevelBinding(decl, source, bindings)
		}
	}
}

func reactCollectDeclaratorBindings(n engine.Node, source []byte, bindings map[string]engine.Node) {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		d := n.Child(i)
		if d.Kind() != "variable_declarator" {
			continue
		}
		name := d.ChildByFieldName("name")
		value := d.ChildByFieldName("value")
		if name == nil || name.Kind() != "identifier" || value == nil {
			continue
		}
		bindings[name.Utf8Text(source)] = value
	}
}

// moduleHasUseClientDirective reports whether root's first non-comment
// top-level statement is an expression_statement whose sole expression is
// the string literal "use client" (either quote style).
func moduleHasUseClientDirective(root engine.Node, source []byte) bool {
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		c := root.Child(i)
		if c.Kind() == "comment" {
			continue
		}
		if c.Kind() != "expression_statement" {
			return false
		}
		str := reactFirstStringChild(c)
		if str == nil {
			return false
		}
		return reactIsUseClientLiteral(str, source)
	}
	return false
}

// reactIsUseClientLiteral reports whether str's raw source text is exactly
// "use client" or 'use client'. tsStringLiteralText is not reused here
// because it delegates to strconv.Unquote, which rejects JS single-quoted
// string literals (a Go-only quoting rule), silently dropping the
// single-quoted spelling of the directive.
func reactIsUseClientLiteral(str engine.Node, source []byte) bool {
	text := str.Utf8Text(source)
	if len(text) < 2 {
		return false
	}
	quote := text[0]
	if (quote != '"' && quote != '\'') || text[len(text)-1] != quote {
		return false
	}
	return text[1:len(text)-1] == "use client"
}

func reactFirstStringChild(n engine.Node) engine.Node {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if c := n.Child(i); c.Kind() == "string" {
			return c
		}
	}
	return nil
}

// reactWalkScope visits n and its descendants in pre-order, but does not
// descend into a nested non-exported PascalCase-named function/arrow's
// subtree: that inner component is excluded entirely from the outer
// candidate's fact walk. A nested non-PascalCase helper's subtree remains
// fully visited -- state/calls inside it attribute to the outer candidate.
func reactWalkScope(n engine.Node, source []byte, visit func(engine.Node)) {
	if n == nil {
		return
	}
	visit(n)
	if reactIsNestedComponent(n, source) {
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		reactWalkScope(n.Child(i), source, visit)
	}
}

func reactIsNestedComponent(n engine.Node, source []byte) bool {
	switch n.Kind() {
	case "function_declaration":
		if name := n.ChildByFieldName("name"); name != nil {
			return isPascalCaseName(name.Utf8Text(source))
		}
		return false
	case "function_expression":
		if name := n.ChildByFieldName("name"); name != nil {
			return isPascalCaseName(name.Utf8Text(source))
		}
		bound := tsBoundIdentifierName(n, source)
		return bound != "<func lit>" && isPascalCaseName(bound)
	case "arrow_function":
		bound := tsBoundIdentifierName(n, source)
		return bound != "<func lit>" && isPascalCaseName(bound)
	default:
		return false
	}
}

func reactScopeContainsJSX(body engine.Node, source []byte) bool {
	found := false
	reactWalkScope(body, source, func(n engine.Node) {
		if found {
			return
		}
		switch n.Kind() {
		case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
			found = true
		}
	})
	return found
}

func reactScopeInvokesHook(body engine.Node, source []byte) bool {
	found := false
	reactWalkScope(body, source, func(n engine.Node) {
		if found || n.Kind() != "call_expression" {
			return
		}
		if reactIsHookCallee(n, source) {
			found = true
		}
	})
	return found
}

func reactIsHookCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		return reactHookNamePattern.MatchString(fn.Utf8Text(source))
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		return reactHookNamePattern.MatchString(prop.Utf8Text(source))
	default:
		return false
	}
}

// reactExtractUseState collects every direct useState()/React.useState()
// call within body's scope (per reactWalkScope's nesting rules), ordered by
// call start_byte ascending.
func reactExtractUseState(body engine.Node, source []byte) []ReactUseStateBinding {
	var calls []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		if isReactUseStateCallee(n, source) {
			calls = append(calls, n)
		}
	})
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].StartByte() < calls[j].StartByte()
	})

	out := make([]ReactUseStateBinding, 0, len(calls))
	for _, call := range calls {
		binding, setter := reactUseStateBindingNames(call, source)
		out = append(out, ReactUseStateBinding{
			Binding:  binding,
			Setter:   setter,
			Location: locationFromNode(call),
		})
	}
	return out
}

// isReactUseStateCallee reports whether call's callee is exactly useState
// or React.useState. Aliased imports (`import { useState as us }`) are a
// deliberately accepted false negative -- this package does not resolve
// import aliases.
func isReactUseStateCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		return fn.Utf8Text(source) == "useState"
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		return obj != nil && prop != nil && obj.Kind() == "identifier" && obj.Utf8Text(source) == "React" && prop.Utf8Text(source) == "useState"
	default:
		return false
	}
}

// reactUseStateBindingNames resolves call's binding/setter pair from its
// enclosing variable_declarator, when call is exactly that declarator's own
// initializer: `const [x, setX] = useState(...)` -> ("x", "setX") when the
// destructure's 2nd element is a plain identifier, else ("x", "");
// `const x = useState(...)` -> ("x", ""); anything else (call not assigned
// via a declarator, or an unsupported name pattern) -> ("", "").
func reactUseStateBindingNames(call engine.Node, source []byte) (string, string) {
	parent := call.Parent()
	if parent == nil || parent.Kind() != "variable_declarator" {
		return "", ""
	}
	value := parent.ChildByFieldName("value")
	if value == nil || !sameNodeSpan(value, call) {
		return "", ""
	}
	name := parent.ChildByFieldName("name")
	if name == nil {
		return "", ""
	}
	switch name.Kind() {
	case "identifier":
		return name.Utf8Text(source), ""
	case "array_pattern":
		elems := reactArrayPatternElements(name)
		binding, setter := "", ""
		if len(elems) > 0 && elems[0] != nil && elems[0].Kind() == "identifier" {
			binding = elems[0].Utf8Text(source)
		}
		if len(elems) > 1 && elems[1] != nil && elems[1].Kind() == "identifier" {
			setter = elems[1].Utf8Text(source)
		}
		return binding, setter
	default:
		return "", ""
	}
}

// reactArrayPatternElements returns n's elements by position, including a
// nil placeholder for an elided element (e.g. `[, setC]`), so that index 0
// is always the binding slot and index 1 the setter slot even when earlier
// elements are holes.
func reactArrayPatternElements(n engine.Node) []engine.Node {
	var out []engine.Node
	count := n.ChildCount()
	pendingElement := false
	for i := 0; i < count; i++ {
		c := n.Child(i)
		switch c.Kind() {
		case "[", "]":
			continue
		case ",":
			if !pendingElement {
				out = append(out, nil)
			}
			pendingElement = false
		default:
			out = append(out, c)
			pendingElement = true
		}
	}
	return out
}

// reactHandlerNamePattern matches an identifier/property/attribute name
// shaped like an event handler binding (onX, handleX), case-insensitively
// on the prefix.
var reactHandlerNamePattern = regexp.MustCompile(`(?i)^(on|handle)`)

// reactJSXOnAttrPattern matches a JSX attribute name shaped like an event
// handler prop (onClick, onSelect, ...).
var reactJSXOnAttrPattern = regexp.MustCompile(`^on[A-Z]`)

// reactDiscriminantOps is the set of equality operators a workspace-branch
// discriminant condition may use.
var reactDiscriminantOps = map[string]bool{"===": true, "!==": true, "==": true, "!=": true}

// reactImperativeAPINames is the closed set of imperative DOM/UI APIs
// recorded; any other callee/property name is ignored.
var reactImperativeAPINames = map[string]bool{
	"getElementById":   true,
	"querySelector":    true,
	"querySelectorAll": true,
	"focus":            true,
	"blur":             true,
	"scrollIntoView":   true,
}

// reactExtractCoordinatedTransitions finds every function/arrow body in
// body's scan set (reactWalkScope) that calls at least two distinct
// useState setters, and returns one ReactCoordinatedTransition per such
// body, ordered by the body's own start_byte. Calls to the same setter
// within one body count once; setters not present in useState (never
// resolved, e.g. destructure fell through to "") never match.
func reactExtractCoordinatedTransitions(body engine.Node, source []byte, useState []ReactUseStateBinding) []ReactCoordinatedTransition {
	setters := map[string]string{}
	for _, u := range useState {
		if u.Setter != "" && u.Binding != "" {
			setters[u.Setter] = u.Binding
		}
	}
	if len(setters) == 0 {
		return nil
	}

	var funcs []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "function_declaration", "function_expression", "arrow_function":
			if reactIsNestedComponent(n, source) {
				return
			}
			funcs = append(funcs, n)
		}
	})

	var out []ReactCoordinatedTransition
	for _, fn := range funcs {
		updated := map[string]struct{}{}
		reactWalkLocalScope(fn.ChildByFieldName("body"), func(n engine.Node) {
			if n.Kind() != "call_expression" {
				return
			}
			callee := n.ChildByFieldName("function")
			if callee == nil || callee.Kind() != "identifier" {
				return
			}
			if binding, ok := setters[callee.Utf8Text(source)]; ok {
				updated[binding] = struct{}{}
			}
		})
		if len(updated) < 2 {
			continue
		}
		bindings := make([]string, 0, len(updated))
		for b := range updated {
			bindings = append(bindings, b)
		}
		sort.Strings(bindings)

		kind, name := reactTransitionKindAndName(fn, source)
		out = append(out, ReactCoordinatedTransition{
			Name:            name,
			Kind:            kind,
			Location:        locationFromNode(fn),
			UpdatedBindings: bindings,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Location.StartByte < out[j].Location.StartByte
	})
	return out
}

// reactWalkLocalScope visits n and its descendants in pre-order but does
// not descend past a nested function/arrow boundary -- unlike
// reactWalkScope, it has no PascalCase-component exception, since any
// function boundary already stops it. Used to attribute calls to the
// function body that directly contains them, not an enclosing one.
func reactWalkLocalScope(n engine.Node, visit func(engine.Node)) {
	if n == nil {
		return
	}
	visit(n)
	switch n.Kind() {
	case "function_declaration", "function_expression", "arrow_function":
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		reactWalkLocalScope(n.Child(i), visit)
	}
}

// reactIsEffectHookCallee reports whether call's callee is exactly one of
// the four supported effect hooks: useEffect, useLayoutEffect,
// React.useEffect, or React.useLayoutEffect.
func reactIsEffectHookCallee(call engine.Node, source []byte) bool {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return false
	}
	switch fn.Kind() {
	case "identifier":
		name := fn.Utf8Text(source)
		return name == "useEffect" || name == "useLayoutEffect"
	case "member_expression":
		obj := fn.ChildByFieldName("object")
		prop := fn.ChildByFieldName("property")
		if obj == nil || prop == nil || obj.Kind() != "identifier" || obj.Utf8Text(source) != "React" {
			return false
		}
		name := prop.Utf8Text(source)
		return name == "useEffect" || name == "useLayoutEffect"
	default:
		return false
	}
}

// reactTransitionKindAndName classifies fn as "effect" (first argument of
// useEffect/useLayoutEffect), "handler" (bound to an on*/handle* name via a
// declarator, assignment, object property, or JSX on* attribute), or
// "callback" (none of the above), and returns the matched name alongside
// ("" for effect and callback).
func reactTransitionKindAndName(fn engine.Node, source []byte) (kind, name string) {
	parent := fn.Parent()
	if parent != nil && parent.Kind() == "arguments" && sameNodeSpan(reactFirstArgumentNode(parent), fn) {
		if call := parent.Parent(); call != nil && call.Kind() == "call_expression" && reactIsEffectHookCallee(call, source) {
			return "effect", ""
		}
	}
	if parent == nil {
		return "callback", ""
	}
	switch parent.Kind() {
	case "variable_declarator":
		if nameNode := parent.ChildByFieldName("name"); nameNode != nil && nameNode.Kind() == "identifier" {
			if n := nameNode.Utf8Text(source); reactHandlerNamePattern.MatchString(n) {
				return "handler", n
			}
		}
	case "assignment_expression":
		if left := parent.ChildByFieldName("left"); left != nil && left.Kind() == "identifier" {
			if n := left.Utf8Text(source); reactHandlerNamePattern.MatchString(n) {
				return "handler", n
			}
		}
	case "pair":
		if key := parent.ChildByFieldName("key"); key != nil && key.Kind() == "property_identifier" {
			if n := key.Utf8Text(source); reactHandlerNamePattern.MatchString(n) {
				return "handler", n
			}
		}
	case "jsx_expression":
		if attr := parent.Parent(); attr != nil && attr.Kind() == "jsx_attribute" {
			if n := reactJSXAttributeNameText(attr, source); reactJSXOnAttrPattern.MatchString(n) {
				return "handler", n
			}
		}
	}
	return "callback", ""
}

// reactJSXAttributeNameText returns attr's name text -- attr's child 0, a
// property_identifier -- or "" when absent. jsx_attribute carries no named
// fields, so children must be accessed positionally.
func reactJSXAttributeNameText(attr engine.Node, source []byte) string {
	if attr.ChildCount() == 0 {
		return ""
	}
	nameNode := attr.Child(0)
	if nameNode.Kind() != "property_identifier" {
		return ""
	}
	return nameNode.Utf8Text(source)
}

// reactJSXAttributeValueNode returns attr's value node -- attr's child 2,
// either a bare string or a jsx_expression -- or nil when attr has no
// value (e.g. a boolean shorthand attribute).
func reactJSXAttributeValueNode(attr engine.Node) engine.Node {
	if attr.ChildCount() < 3 {
		return nil
	}
	return attr.Child(2)
}

func reactJSXAttributes(n engine.Node) []engine.Node {
	var out []engine.Node
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if c := n.Child(i); c.Kind() == "jsx_attribute" {
			out = append(out, c)
		}
	}
	return out
}

// reactJSXExpressionInner returns the expression node wrapped by a
// jsx_expression ("{" expr "}"), skipping the brace tokens, or nil when
// none is present.
func reactJSXExpressionInner(jsxExpr engine.Node) engine.Node {
	count := jsxExpr.ChildCount()
	for i := 0; i < count; i++ {
		c := jsxExpr.Child(i)
		switch c.Kind() {
		case "{", "}":
			continue
		default:
			return c
		}
	}
	return nil
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

// reactExtractWorkspaceBranches collects ReactWorkspaceBranch entries from
// two disjoint constructs: discriminant ternary/if-else-if chains gated at
// >=3 JSX-bearing branches per chain, and role="tabpanel" JSX elements
// (ungated). Results are de-duplicated by the branch's own Location.StartByte
// and ordered by that same start_byte.
func reactExtractWorkspaceBranches(body engine.Node, source []byte) []ReactWorkspaceBranch {
	var branches []ReactWorkspaceBranch
	seen := map[uint]struct{}{}
	addBranch := func(b ReactWorkspaceBranch) {
		if _, dup := seen[b.Location.StartByte]; dup {
			return
		}
		seen[b.Location.StartByte] = struct{}{}
		branches = append(branches, b)
	}

	consumed := map[[2]uint]struct{}{}
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "ternary_expression", "if_statement":
		default:
			return
		}
		key := [2]uint{n.StartByte(), n.EndByte()}
		if _, done := consumed[key]; done {
			return
		}
		base, ok := reactDiscriminantConditionBase(n, source)
		if !ok {
			return
		}
		chainBranches, chainNodes := reactCollectDiscriminantChain(n, base, source)
		for _, cn := range chainNodes {
			consumed[[2]uint{cn.StartByte(), cn.EndByte()}] = struct{}{}
		}
		if len(chainBranches) < 3 {
			return
		}
		for _, b := range chainBranches {
			addBranch(b)
		}
	})

	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "jsx_opening_element", "jsx_self_closing_element":
		default:
			return
		}
		if b, ok := reactTabpanelBranch(n, source); ok {
			addBranch(b)
		}
	})

	sort.SliceStable(branches, func(i, j int) bool {
		return branches[i].Location.StartByte < branches[j].Location.StartByte
	})
	return branches
}

// reactDiscriminantConditionBase reports whether n's "condition" field is a
// binary_expression testing equality/inequality between a discriminant base
// (a plain identifier, or a member_expression -- whose full text, not just
// its property, is used as the base so structurally distinct discriminants
// like props.v and other.v never compare equal) and a string/number
// literal, and returns that base text.
func reactDiscriminantConditionBase(n engine.Node, source []byte) (string, bool) {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return "", false
	}
	if !reactDiscriminantOps[tsBinaryOp(cond)] {
		return "", false
	}
	left := cond.ChildByFieldName("left")
	right := cond.ChildByFieldName("right")
	if left == nil || right == nil {
		return "", false
	}
	if base, ok := reactDiscriminantBase(left, right, source); ok {
		return base, true
	}
	return reactDiscriminantBase(right, left, source)
}

func reactDiscriminantBase(baseSide, literalSide engine.Node, source []byte) (string, bool) {
	if literalSide == nil || (literalSide.Kind() != "string" && literalSide.Kind() != "number") {
		return "", false
	}
	switch baseSide.Kind() {
	case "identifier":
		return baseSide.Utf8Text(source), true
	case "member_expression":
		return baseSide.Utf8Text(source), true
	}
	return "", false
}

// reactCollectDiscriminantChain walks n forward through same-base chained
// ternary/if-else-if branches and returns the JSX-bearing branches found
// plus every chain node visited (regardless of JSX-bearing outcome), so the
// caller can mark the whole chain consumed even when the gate fails it.
func reactCollectDiscriminantChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	switch n.Kind() {
	case "ternary_expression":
		return reactCollectTernaryChain(n, base, source)
	case "if_statement":
		return reactCollectIfChain(n, base, source)
	default:
		return nil, nil
	}
}

func reactCollectTernaryChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	var branches []ReactWorkspaceBranch
	var nodes []engine.Node
	cur := n
	for cur != nil && cur.Kind() == "ternary_expression" {
		curBase, ok := reactDiscriminantConditionBase(cur, source)
		if !ok || curBase != base {
			break
		}
		nodes = append(nodes, cur)
		if b, ok := reactTernaryBranch(cur, source); ok {
			branches = append(branches, b)
		}
		alt := unwrapTSParen(cur.ChildByFieldName("alternative"))
		if alt != nil && alt.Kind() == "ternary_expression" {
			cur = alt
			continue
		}
		break
	}
	return branches, nodes
}

func reactCollectIfChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	var branches []ReactWorkspaceBranch
	var nodes []engine.Node
	cur := n
	for cur != nil && cur.Kind() == "if_statement" {
		curBase, ok := reactDiscriminantConditionBase(cur, source)
		if !ok || curBase != base {
			break
		}
		nodes = append(nodes, cur)
		if b, ok := reactIfBranch(cur, source); ok {
			branches = append(branches, b)
		}

		alt := cur.ChildByFieldName("alternative")
		if alt != nil && alt.Kind() == "else_clause" {
			alt = tsElseClauseInner(alt)
		}
		if alt != nil && alt.Kind() == "if_statement" {
			cur = alt
			continue
		}
		break
	}
	return branches, nodes
}

func reactTernaryBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	cons := unwrapTSParen(n.ChildByFieldName("consequence"))
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(n, cons, source),
		Location: locationFromNode(cons),
	}, true
}

func reactIfBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	cons := n.ChildByFieldName("consequence")
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(n, cons, source),
		Location: locationFromNode(cons),
	}, true
}

// reactWorkspaceBranchLabel applies the label precedence rule: the
// discriminant condition's literal text first, else a capitalized primary
// JSX child's tag name, else "".
func reactWorkspaceBranchLabel(condHolder, cons engine.Node, source []byte) string {
	if lbl := reactDiscriminantLiteralLabel(condHolder, source); lbl != "" {
		return lbl
	}
	return reactCapitalizedJSXChildLabel(cons, source)
}

func reactDiscriminantLiteralLabel(n engine.Node, source []byte) string {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return ""
	}
	if lbl, ok := reactLiteralLabelText(cond.ChildByFieldName("left"), source); ok {
		return lbl
	}
	if lbl, ok := reactLiteralLabelText(cond.ChildByFieldName("right"), source); ok {
		return lbl
	}
	return ""
}

func reactLiteralLabelText(n engine.Node, source []byte) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind() {
	case "string":
		return reactBareStringText(n, source)
	case "number":
		return n.Utf8Text(source), true
	default:
		return "", false
	}
}

func reactCapitalizedJSXChildLabel(cons engine.Node, source []byte) string {
	el := reactPrimaryJSXElement(cons)
	if el == nil {
		return ""
	}
	name := reactJSXElementTagName(el, source)
	if name == "" || !isPascalCaseName(name) {
		return ""
	}
	return name
}

func reactPrimaryJSXElement(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element":
		return n
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if el := reactPrimaryJSXElement(n.Child(i)); el != nil {
			return el
		}
	}
	return nil
}

func reactJSXElementTagName(n engine.Node, source []byte) string {
	switch n.Kind() {
	case "jsx_element":
		open := n.ChildByFieldName("open_tag")
		if open == nil {
			return ""
		}
		if name := open.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return ""
	case "jsx_self_closing_element":
		if name := n.ChildByFieldName("name"); name != nil {
			return name.Utf8Text(source)
		}
		return ""
	default:
		return ""
	}
}

func reactContainsJSX(n engine.Node) bool {
	if n == nil {
		return false
	}
	switch n.Kind() {
	case "jsx_element", "jsx_self_closing_element", "jsx_fragment":
		return true
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if reactContainsJSX(n.Child(i)) {
			return true
		}
	}
	return false
}

// reactTabpanelBranch reports whether n is a JSX opening/self-closing
// element carrying role="tabpanel", and if so returns its branch: label is
// its aria-label attribute's string value, else its id attribute's string
// value, else the literal "tabpanel".
func reactTabpanelBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	attrs := reactJSXAttributes(n)
	roleOK := false
	for _, attr := range attrs {
		if reactJSXAttributeNameText(attr, source) != "role" {
			continue
		}
		if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok && s == "tabpanel" {
			roleOK = true
		}
	}
	if !roleOK {
		return ReactWorkspaceBranch{}, false
	}

	label := ""
	for _, attr := range attrs {
		if reactJSXAttributeNameText(attr, source) != "aria-label" {
			continue
		}
		if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok {
			label = s
		}
	}
	if label == "" {
		for _, attr := range attrs {
			if reactJSXAttributeNameText(attr, source) != "id" {
				continue
			}
			if s, ok := reactBareStringText(reactJSXAttributeValueNode(attr), source); ok {
				label = s
			}
		}
	}
	if label == "" {
		label = "tabpanel"
	}
	return ReactWorkspaceBranch{Label: label, Location: locationFromNode(n)}, true
}

// reactExtractImperativeUI collects one ReactImperativeUICall per
// call_expression in body's scan set whose resolved API name (the callee
// identifier, or a member_expression callee's property -- covering both
// `.` and `?.` uniformly, since optional chaining has no distinct node
// kind) is in the closed reactImperativeAPINames set, ordered by the call's
// own start_byte.
func reactExtractImperativeUI(body engine.Node, source []byte) []ReactImperativeUICall {
	var calls []engine.Node
	reactWalkScope(body, source, func(n engine.Node) {
		if n.Kind() != "call_expression" {
			return
		}
		if reactImperativeAPI(n, source) != "" {
			calls = append(calls, n)
		}
	})
	sort.SliceStable(calls, func(i, j int) bool {
		return calls[i].StartByte() < calls[j].StartByte()
	})

	out := make([]ReactImperativeUICall, 0, len(calls))
	for _, c := range calls {
		out = append(out, ReactImperativeUICall{
			API:      reactImperativeAPI(c, source),
			Location: locationFromNode(c),
		})
	}
	return out
}

func reactImperativeAPI(call engine.Node, source []byte) string {
	fn := call.ChildByFieldName("function")
	if fn == nil {
		return ""
	}
	var api string
	switch fn.Kind() {
	case "member_expression":
		prop := fn.ChildByFieldName("property")
		if prop == nil {
			return ""
		}
		api = prop.Utf8Text(source)
	case "identifier":
		api = fn.Utf8Text(source)
	default:
		return ""
	}
	if reactImperativeAPINames[api] {
		return api
	}
	return ""
}

// reactExtractSharedPanelDeps groups every `attr={identifier}` JSX
// attribute (identifier value, not a member expression/spread/literal)
// found on an uppercase-tag JSX element in body's scan set by identifier
// name, and returns one ReactSharedPanelDep per identifier referenced this
// way by >=2 distinct tag names, ordered by Name.
func reactExtractSharedPanelDeps(body engine.Node, source []byte) []ReactSharedPanelDep {
	depTags := map[string]map[string]struct{}{}

	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "jsx_opening_element", "jsx_self_closing_element":
		default:
			return
		}
		nameNode := n.ChildByFieldName("name")
		if nameNode == nil {
			return
		}
		tag := nameNode.Utf8Text(source)
		if !isPascalCaseName(tag) {
			return
		}
		for _, attr := range reactJSXAttributes(n) {
			val := reactJSXAttributeValueNode(attr)
			if val == nil || val.Kind() != "jsx_expression" {
				continue
			}
			inner := reactJSXExpressionInner(val)
			if inner == nil || inner.Kind() != "identifier" {
				continue
			}
			idName := inner.Utf8Text(source)
			if depTags[idName] == nil {
				depTags[idName] = map[string]struct{}{}
			}
			depTags[idName][tag] = struct{}{}
		}
	})

	var names []string
	for name, tags := range depTags {
		if len(tags) >= 2 {
			names = append(names, name)
		}
	}
	sort.Strings(names)

	out := make([]ReactSharedPanelDep, 0, len(names))
	for _, name := range names {
		tags := depTags[name]
		panels := make([]string, 0, len(tags))
		for t := range tags {
			panels = append(panels, t)
		}
		sort.Strings(panels)
		out = append(out, ReactSharedPanelDep{Name: name, Panels: panels})
	}
	return out
}

func isPascalCaseName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
