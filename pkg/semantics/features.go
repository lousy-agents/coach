package semantics

import (
	"regexp"
	"strings"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// constructorFuncNameRe matches function names that look like Go
// constructors: "New" followed by an uppercase letter, digit, or
// underscore (e.g. NewFoo, New2, New_thing), or "New" alone. It
// deliberately does not match names like "Newton", where the character
// after "New" is a lowercase letter.
var constructorFuncNameRe = regexp.MustCompile(`^New([A-Z0-9_]|$)`)

// computeGoFeatures walks root exactly once, producing both the structural
// metrics and the pattern findings for the file. A single traversal is used
// because nesting depth cannot be expressed as a Tree-sitter query -- doing
// metrics and findings together keeps results deterministic and cheap
// (avoids re-walking the tree per concern).
func computeGoFeatures(root engine.Node, source []byte) (StructuralMetrics, []Finding) {
	c := &featureCollector{}
	c.walk(root, source, 0, false)
	return c.metrics, c.findings
}

// featureCollector accumulates StructuralMetrics and Findings during a
// single pre-order walk of the tree.
type featureCollector struct {
	metrics  StructuralMetrics
	findings []Finding
}

// walk visits n and its descendants in pre-order, incrementing metrics
// counters for the node kinds AC-3.3 tracks. blockDepth counts nested
// "block" nodes seen so far within the current function/method body (0
// outside any body); inFunc reports whether the walk is currently inside a
// function_declaration or method_declaration, since nesting depth (AC-3.4)
// is only measured inside those bodies.
func (c *featureCollector) walk(n engine.Node, source []byte, blockDepth int, inFunc bool) {
	if n == nil {
		return
	}

	switch n.Kind() {
	case "if_statement":
		c.metrics.Ifs++
	case "for_statement":
		c.metrics.Fors++
	case "expression_switch_statement":
		c.metrics.ExprSwitches++
	case "type_switch_statement":
		c.metrics.TypeSwitches++
	case "select_statement":
		c.metrics.Selects++
	case "function_declaration":
		c.metrics.Functions++
		c.checkConstructorFunc(n, source)
		c.checkPointerReturn(n, source)
		c.checkMutatesInput(n, source)
		inFunc = true
		blockDepth = 0
	case "method_declaration":
		c.metrics.Methods++
		c.checkPointerReturn(n, source)
		c.checkMutatesInput(n, source)
		inFunc = true
		blockDepth = 0
	case "block":
		if inFunc {
			blockDepth++
			if blockDepth > c.metrics.MaxNestingDepth {
				c.metrics.MaxNestingDepth = blockDepth
			}
		}
	}

	count := n.ChildCount()
	for i := 0; i < count; i++ {
		c.walk(n.Child(i), source, blockDepth, inFunc)
	}
}

// checkConstructorFunc emits a "constructor_func" Finding (AC-3.5) if decl's
// name field matches constructorFuncNameRe.
func (c *featureCollector) checkConstructorFunc(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(source)
	if !constructorFuncNameRe.MatchString(name) {
		return
	}
	c.findings = append(c.findings, Finding{
		Kind:     "constructor_func",
		Name:     name,
		Location: locationFromNode(decl),
	})
}

// checkPointerReturn emits a "pointer_return" Finding (AC-3.6) if decl's
// result field contains a pointer_type, either directly (a single unnamed
// pointer return value) or among a parameter_list's parameter_declaration
// types (multiple and/or named return values).
func (c *featureCollector) checkPointerReturn(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	if nameNode == nil {
		return
	}
	result := decl.ChildByFieldName("result")
	if result == nil || !resultHasPointerType(result) {
		return
	}
	c.findings = append(c.findings, Finding{
		Kind:     "pointer_return",
		Name:     nameNode.Utf8Text(source),
		Location: locationFromNode(decl),
	})
}

// resultHasPointerType reports whether a function/method's result field
// contains a pointer_type anywhere in its subtree: as the result node
// itself (a single unnamed pointer return value), as a parameter_list's
// parameter_declaration type (multiple and/or named return values), or
// nested inside a composite type such as a slice, map value, or channel
// element (e.g. []*T, map[string]*T, chan *T). A full descendant search is
// used rather than checking only the direct result/type node, since Go
// permits pointer_type at any depth within a composite result type.
func resultHasPointerType(result engine.Node) bool {
	if result.Kind() == "pointer_type" {
		return true
	}
	count := result.ChildCount()
	for i := 0; i < count; i++ {
		if resultHasPointerType(result.Child(i)) {
			return true
		}
	}
	return false
}

// checkMutatesInput emits one "mutates_input" Finding per distinct
// (parameter, mutation-expression location) pair where decl's body writes
// through a syntactically pointer/map/slice-typed parameter, either via a
// selector on the parameter or a dereference of it (cfg.Name = x,
// (*cfg).Name = x) or via index assignment on a map/slice parameter
// (values[k] = x, items[i] = x). Plain reassignment of the parameter
// variable itself (cfg = other) is a rebind, not a caller-visible mutation,
// and is deliberately excluded.
func (c *featureCollector) checkMutatesInput(decl engine.Node, source []byte) {
	nameNode := decl.ChildByFieldName("name")
	params := decl.ChildByFieldName("parameters")
	body := decl.ChildByFieldName("body")
	if nameNode == nil || params == nil || body == nil {
		return
	}
	funcName := nameNode.Utf8Text(source)
	mutableParams := mutableParamTypes(params, source)
	if len(mutableParams) == 0 {
		return
	}

	seen := map[mutatesInputKey]bool{}
	c.findAssignments(body, source, funcName, mutableParams, seen)
}

// mutatesInputKey dedupes findings by (parameter, mutation-expression
// location) per AC-5: repeated writes to the same parameter through the
// same source location must not produce duplicate findings.
type mutatesInputKey struct {
	paramName string
	startByte uint
	endByte   uint
}

// paramMutKind classifies a declared parameter's syntactic type for
// mutates_input purposes. Selector/dereference writes (cfg.Name = x,
// (*cfg).Name = x) are only caller-visible through a pointer, and index
// writes (values[k] = x, items[i] = x) are only caller-visible through a
// map or slice -- collapsing these into a single bool would let a
// selector write on a map/slice parameter, or an index write on a
// (non-array) pointer parameter, be misreported as mutates_input even
// though neither is a caller-visible write for that parameter's actual
// type.
type paramMutKind uint8

const (
	// paramNotMutable is also the zero value, so a parameter absent from
	// a paramMutKind map (or explicitly recorded as such) is never
	// mistaken for a pointer/collection parameter.
	paramNotMutable paramMutKind = iota
	paramMutPointer
	paramMutCollection
)

// mutableParamTypes maps each declared parameter identifier to its
// paramMutKind, derived from whether its syntactic type is a pointer_type
// (paramMutPointer) or a map_type/slice_type (paramMutCollection). Go
// allows a single parameter_declaration to bind multiple names to one
// shared type (func f(a, b *T)), so every identifier child of each
// parameter_declaration is collected, not just the first.
func mutableParamTypes(params engine.Node, source []byte) map[string]paramMutKind {
	result := map[string]paramMutKind{}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		decl := params.Child(i)
		if decl.Kind() != "parameter_declaration" {
			continue
		}
		typeNode := decl.ChildByFieldName("type")
		kind := paramNotMutable
		if typeNode != nil {
			switch typeNode.Kind() {
			case "pointer_type":
				kind = paramMutPointer
			case "map_type", "slice_type":
				kind = paramMutCollection
			}
		}

		declCount := decl.ChildCount()
		for j := 0; j < declCount; j++ {
			nameChild := decl.Child(j)
			if nameChild.Kind() != "identifier" {
				continue
			}
			result[nameChild.Utf8Text(source)] = kind
		}
	}
	return result
}

// findAssignments walks n's subtree looking for assignment_statement nodes
// whose left-hand-side targets write through a mutable parameter, and
// records a deduplicated "mutates_input" Finding for each. It does not
// descend specially into nested function_declaration/func_literal bodies:
// a closure's own mutation of an outer mutable parameter is still a
// caller-visible mutation of that parameter and is deliberately still
// reported.
func (c *featureCollector) findAssignments(n engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind, seen map[mutatesInputKey]bool) {
	if n == nil {
		return
	}

	if n.Kind() == "type_switch_statement" {
		// The type-switch alias (e.g. `cfg` in `switch cfg := v.(type)`) is
		// scoped only to this switch statement's own subtree, so it must
		// shadow mutableParams for the recursive descent below but must not
		// leak into the shadowing applied to the enclosing block's later
		// siblings at the bottom of this function.
		mutableParams = shadowNames(mutableParams, identifiersInNodeField(n, "alias", source))
	}

	if n.Kind() == "assignment_statement" {
		left := n.ChildByFieldName("left")
		if left != nil {
			targetCount := left.ChildCount()
			for i := 0; i < targetCount; i++ {
				c.checkAssignmentTarget(left.Child(i), source, funcName, mutableParams, seen)
			}
		}
	}
	if n.Kind() == "inc_statement" || n.Kind() == "dec_statement" {
		c.checkAssignmentTarget(updateStatementTarget(n), source, funcName, mutableParams, seen)
	}
	if n.Kind() == "func_literal" {
		if params := n.ChildByFieldName("parameters"); params != nil {
			mutableParams = shadowParamTypes(mutableParams, params, source)
		}
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		c.findAssignments(child, source, funcName, mutableParams, seen)
		mutableParams = shadowLocalDeclarations(mutableParams, child, source)
	}
}

func updateStatementTarget(n engine.Node) engine.Node {
	if n == nil {
		return nil
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		switch child.Kind() {
		case "selector_expression", "index_expression", "unary_expression":
			return child
		}
	}
	return nil
}

// shadowLocalDeclarations returns a copy of outer with any entries shadowed by
// local declarations or direct parameter rebindings in n removed. It is
// applied while walking child nodes in source order so a short variable
// declaration such as `cfg := &Config{}` or a parameter rebind such as
// `cfg = other` shadows an outer parameter only for subsequent source nodes.
func shadowLocalDeclarations(outer map[string]paramMutKind, n engine.Node, source []byte) map[string]paramMutKind {
	if len(outer) == 0 || n == nil {
		return outer
	}

	var names map[string]bool
	switch n.Kind() {
	case "short_var_declaration":
		names = identifiersInNodeField(n, "left", source)
	case "assignment_statement":
		names = reboundIdentifiersInAssignment(n, source)
	case "for_clause":
		names = identifiersDeclaredInSubtree(n, source)
	case "range_clause":
		names = identifiersInNodeField(n, "left", source)
	case "type_switch_header", "type_switch_guard":
		names = typeSwitchGuardIdentifiers(n, source)
	case "var_declaration":
		names = identifiersInVarDeclaration(n, source)
	default:
		return outer
	}
	return shadowNames(outer, names)
}

// shadowNames returns a copy of outer with any entries whose name appears in
// names removed.
func shadowNames(outer map[string]paramMutKind, names map[string]bool) map[string]paramMutKind {
	if len(outer) == 0 || len(names) == 0 {
		return outer
	}

	shadowed := make(map[string]paramMutKind, len(outer))
	for name, kind := range outer {
		if names[name] {
			continue
		}
		shadowed[name] = kind
	}
	return shadowed
}

func reboundIdentifiersInAssignment(n engine.Node, source []byte) map[string]bool {
	left := n.ChildByFieldName("left")
	if left == nil {
		return nil
	}
	names := map[string]bool{}
	count := left.ChildCount()
	for i := 0; i < count; i++ {
		child := left.Child(i)
		if child.Kind() == "identifier" {
			names[child.Utf8Text(source)] = true
		}
	}
	return names
}

func identifiersDeclaredInSubtree(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		switch node.Kind() {
		case "short_var_declaration":
			for name := range identifiersInNodeField(node, "left", source) {
				names[name] = true
			}
			return
		case "var_declaration":
			for name := range identifiersInVarDeclaration(node, source) {
				names[name] = true
			}
			return
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			collect(node.Child(i))
		}
	}
	collect(n)
	return names
}

func typeSwitchGuardIdentifiers(n engine.Node, source []byte) map[string]bool {
	if n == nil {
		return nil
	}
	if n.Kind() == "type_switch_guard" {
		text := n.Utf8Text(source)
		if !strings.Contains(text, ":=") {
			return nil
		}
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			child := n.Child(i)
			if child.Kind() == "identifier" {
				return map[string]bool{child.Utf8Text(source): true}
			}
		}
		return nil
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if names := typeSwitchGuardIdentifiers(n.Child(i), source); len(names) > 0 {
			return names
		}
	}
	return nil
}

func identifiersInNodeField(n engine.Node, field string, source []byte) map[string]bool {
	child := n.ChildByFieldName(field)
	if child == nil {
		return nil
	}
	names := map[string]bool{}
	collectIdentifiers(child, source, names)
	return names
}

func identifiersInVarDeclaration(n engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	var collect func(engine.Node)
	collect = func(node engine.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "var_spec" {
			if name := node.ChildByFieldName("name"); name != nil {
				collectIdentifiers(name, source, names)
			}
			return
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			collect(node.Child(i))
		}
	}
	collect(n)
	return names
}

func collectIdentifiers(n engine.Node, source []byte, names map[string]bool) {
	if n == nil {
		return
	}
	if n.Kind() == "identifier" {
		names[n.Utf8Text(source)] = true
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectIdentifiers(n.Child(i), source, names)
	}
}

// shadowParamTypes returns a copy of outer with any entries shadowed by
// literalParams (a func_literal's own "parameters" field) removed, so that
// a closure declaring its own parameter with the same name as an outer
// function's parameter (e.g. an outer `cfg` shadowed by a closure's own
// `func(cfg *Config){ ... }`) is never misattributed to the outer
// parameter: the closure's binding is a distinct variable per normal Go
// scoping, and this single-pass walk has no separate owner name to
// attribute the closure's own mutations to, so those are simply not
// reported rather than being reported against the wrong (outer) name.
// Outer parameters not redeclared by the literal are left untouched, since
// the walk is still inside their scope.
func shadowParamTypes(outer map[string]paramMutKind, literalParams engine.Node, source []byte) map[string]paramMutKind {
	ownParams := parameterNames(literalParams, source)
	if len(ownParams) == 0 {
		return outer
	}
	shadowed := make(map[string]paramMutKind, len(outer))
	for name, kind := range outer {
		if ownParams[name] {
			continue
		}
		shadowed[name] = kind
	}
	return shadowed
}

func parameterNames(params engine.Node, source []byte) map[string]bool {
	names := map[string]bool{}
	if params == nil {
		return names
	}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		collectParameterNames(params.Child(i), source, names)
	}
	return names
}

func collectParameterNames(n engine.Node, source []byte, names map[string]bool) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "parameter_declaration", "variadic_parameter_declaration":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			child := n.Child(i)
			if child.Kind() == "identifier" {
				names[child.Utf8Text(source)] = true
			}
		}
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		collectParameterNames(n.Child(i), source, names)
	}
}

// checkAssignmentTarget inspects a single assignment target (one of an
// assignment_statement's possibly-multiple left-hand-side expressions) and
// emits a "mutates_input" Finding if it writes through a mutable parameter.
func (c *featureCollector) checkAssignmentTarget(target engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind, seen map[mutatesInputKey]bool) {
	if target == nil {
		return
	}

	// requiredKind is the only paramMutKind that makes a DIRECT selector or
	// index target (the parameter itself, optionally through one "(*p)"
	// dereference hop -- exactly the shapes the acceptance criteria name:
	// cfg.Name, (*cfg).Name, values[k], items[i]) a caller-visible write:
	// selector/dereference is only caller-visible through a pointer
	// parameter, and index is only caller-visible through a map/slice
	// parameter. A map/slice parameter's direct selector write, or a
	// pointer parameter's direct index write, is not detected.
	//
	// A NESTED target (reached through at least one additional
	// selector/index hop beyond the direct base, e.g. cfg.Sub.Name or
	// cfg.Items[0]) is checked more permissively -- any mutable root kind
	// is accepted regardless of the target's own shape -- because the
	// intermediate field/element's own type (e.g. that Items is a slice)
	// is not visible without resolving another type's declaration
	// elsewhere in the file, which is out of scope for this syntax-only
	// detector; the root parameter being a pointer (or map/slice) at all
	// already establishes that state reachable through it is
	// caller-visible.
	var paramName string
	var requiredKind paramMutKind
	switch target.Kind() {
	case "parenthesized_expression":
		inner := parenthesizedInner(target)
		if inner == nil {
			return
		}
		c.checkAssignmentTarget(inner, source, funcName, mutableParams, seen)
		return
	case "selector_expression":
		requiredKind = paramMutPointer
	case "index_expression":
		requiredKind = paramMutCollection
	case "unary_expression":
		op := target.ChildByFieldName("operator")
		if op == nil || op.Utf8Text(source) != "*" {
			return
		}
		operand := target.ChildByFieldName("operand")
		if operand == nil || operand.Kind() != "identifier" {
			return
		}
		paramName = operand.Utf8Text(source)
		if mutableParams[paramName] != paramMutPointer {
			return
		}
		loc := locationFromNode(target)
		key := mutatesInputKey{paramName: paramName, startByte: loc.StartByte, endByte: loc.EndByte}
		if seen[key] {
			return
		}
		seen[key] = true
		c.findings = append(c.findings, newMutatesInputFinding(funcName, paramName, target, source))
		return
	default:
		// Plain identifier targets (cfg = other) rebind the local
		// parameter variable rather than writing through it, and any
		// other target kind is out of scope for this detector.
		return
	}
	operand := target.ChildByFieldName("operand")
	if operand == nil {
		return
	}
	base, nested := selectorBaseIdentifier(operand, source)
	if base == nil {
		return
	}
	paramName = base.Utf8Text(source)

	kind := mutableParams[paramName]
	if nested {
		if kind == paramNotMutable {
			return
		}
	} else if kind != requiredKind {
		return
	}

	loc := locationFromNode(target)
	key := mutatesInputKey{paramName: paramName, startByte: loc.StartByte, endByte: loc.EndByte}
	if seen[key] {
		return
	}
	seen[key] = true

	c.findings = append(c.findings, newMutatesInputFinding(funcName, paramName, target, source))
}

// selectorBaseIdentifier resolves a selector_expression's operand down to
// the root identifier it ultimately reads, walking through any chain of
// nested selector_expression/index_expression operands (cfg.Sub.Name,
// cfg.Items[0].Name) and the parenthesized-unary-dereference case
// ((*cfg).Name, (*cfg.Sub).Name), iterating until it reaches a plain
// identifier -- the root -- or determines there is no such root (e.g. the
// chain bottoms out in a function call like f().Name), in which case base
// is nil.
//
// nested reports whether reaching that root required stepping through at
// least one selector_expression/index_expression hop beyond the "direct"
// shapes the acceptance criteria name explicitly: a bare identifier
// (cfg.Name, where operand IS the parameter) or a single parenthesized
// dereference of one ((*cfg).Name). Those two direct shapes report
// nested == false; anything requiring an additional hop (cfg.Sub.Name,
// cfg.Items[0]) reports nested == true. checkAssignmentTarget uses this to
// require an exact selector<->pointer / index<->collection kind match only
// for direct targets, where the acceptance criteria pin down the required
// parameter kind precisely -- a nested target's intermediate field/element
// type is not visible to this syntax-only detector, so it is checked more
// permissively (see checkAssignmentTarget's comment).
func selectorBaseIdentifier(operand engine.Node, source []byte) (base engine.Node, nested bool) {
	switch operand.Kind() {
	case "identifier":
		return operand, false
	case "parenthesized_expression":
		if inner := derefOperand(operand, source); inner != nil && inner.Kind() == "identifier" {
			return inner, false
		}
		if inner := parenthesizedInner(operand); inner != nil && inner.Kind() == "identifier" {
			return inner, false
		}
	}
	for operand != nil {
		switch operand.Kind() {
		case "identifier":
			return operand, true
		case "selector_expression", "index_expression":
			operand = operand.ChildByFieldName("operand")
		case "parenthesized_expression":
			if inner := derefOperand(operand, source); inner != nil {
				operand = inner
			} else {
				operand = parenthesizedInner(operand)
			}
		default:
			return nil, false
		}
	}
	return nil, false
}

func parenthesizedInner(paren engine.Node) engine.Node {
	count := paren.ChildCount()
	for i := 0; i < count; i++ {
		child := paren.Child(i)
		if child.Kind() != "(" && child.Kind() != ")" {
			return child
		}
	}
	return nil
}

// derefOperand returns the operand of the parenthesized unary "*"
// (dereference) expression nested directly inside a parenthesized_expression
// ((*cfg) -> cfg, (*cfg.Sub) -> cfg.Sub), or nil if paren does not wrap
// exactly that shape. A unary_expression's "operator" field must be checked
// by source text, not just by the presence of a unary_expression node: Go's
// grammar uses the same unary_expression node for "*", "&", "!", "-", "+",
// "^", and "<-", so a parenthesized non-dereference unary expression like
// (-x), (!x), or (&x) must not be mis-resolved as if it were (*x).
func derefOperand(paren engine.Node, source []byte) engine.Node {
	count := paren.ChildCount()
	for i := 0; i < count; i++ {
		child := paren.Child(i)
		if child.Kind() != "unary_expression" {
			continue
		}
		op := child.ChildByFieldName("operator")
		if op == nil || op.Utf8Text(source) != "*" {
			continue
		}
		if inner := child.ChildByFieldName("operand"); inner != nil {
			return inner
		}
	}
	return nil
}

// newMutatesInputFinding builds a "mutates_input" Finding (Story 1/3) for a
// mutation of parameter paramName within function/method funcName, located
// at evidence's own source span so tooling can point directly at the
// mutating expression rather than the enclosing declaration.
func newMutatesInputFinding(funcName, paramName string, evidence engine.Node, source []byte) Finding {
	return Finding{
		Kind:           "mutates_input",
		Name:           funcName + ":" + paramName,
		Location:       locationFromNode(evidence),
		Confidence:     "medium",
		Evidence:       evidence.Utf8Text(source),
		Recommendation: "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
		SuggestedSkill: "refactor-hidden-mutation",
	}
}
