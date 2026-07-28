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
		for name, node := range reactModuleBindingsFrom(root.Child(i), source) {
			bindings[name] = node
		}
	}
	return bindings
}

func reactModuleBindingsFrom(n engine.Node, source []byte) map[string]engine.Node {
	switch n.Kind() {
	case "function_declaration":
		name := n.ChildByFieldName("name")
		if name == nil {
			return nil
		}
		return map[string]engine.Node{name.Utf8Text(source): n}
	case "lexical_declaration", "variable_declaration":
		return reactDeclaratorBindingMap(n, source)
	case "export_statement":
		decl := n.ChildByFieldName("declaration")
		if decl == nil {
			return nil
		}
		return reactModuleBindingsFrom(decl, source)
	default:
		return nil
	}
}

func reactDeclaratorBindingMap(n engine.Node, source []byte) map[string]engine.Node {
	out := map[string]engine.Node{}
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
		out[name.Utf8Text(source)] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
			out = reactArrayPatternOnComma(out, pendingElement)
			pendingElement = false
		default:
			out = append(out, c)
			pendingElement = true
		}
	}
	return out
}

func reactArrayPatternOnComma(out []engine.Node, pendingElement bool) []engine.Node {
	if pendingElement {
		return out
	}
	return append(out, nil)
}

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

func isPascalCaseName(name string) bool {
	if name == "" {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}
