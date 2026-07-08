package semantics

import (
	"regexp"

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

// mutableParamTypes maps each declared parameter identifier to whether its
// syntactic type is a pointer_type, map_type, or slice_type. Go allows a
// single parameter_declaration to bind multiple names to one shared type
// (func f(a, b *T)), so every identifier child of each parameter_declaration
// is collected, not just the first.
func mutableParamTypes(params engine.Node, source []byte) map[string]bool {
	result := map[string]bool{}
	count := params.ChildCount()
	for i := 0; i < count; i++ {
		decl := params.Child(i)
		if decl.Kind() != "parameter_declaration" {
			continue
		}
		typeNode := decl.ChildByFieldName("type")
		mutable := typeNode != nil &&
			(typeNode.Kind() == "pointer_type" || typeNode.Kind() == "map_type" || typeNode.Kind() == "slice_type")

		declCount := decl.ChildCount()
		for j := 0; j < declCount; j++ {
			nameChild := decl.Child(j)
			if nameChild.Kind() != "identifier" {
				continue
			}
			result[nameChild.Utf8Text(source)] = mutable
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
func (c *featureCollector) findAssignments(n engine.Node, source []byte, funcName string, mutableParams map[string]bool, seen map[mutatesInputKey]bool) {
	if n == nil {
		return
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
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		c.findAssignments(n.Child(i), source, funcName, mutableParams, seen)
	}
}

// checkAssignmentTarget inspects a single assignment target (one of an
// assignment_statement's possibly-multiple left-hand-side expressions) and
// emits a "mutates_input" Finding if it writes through a mutable parameter.
func (c *featureCollector) checkAssignmentTarget(target engine.Node, source []byte, funcName string, mutableParams map[string]bool, seen map[mutatesInputKey]bool) {
	if target == nil {
		return
	}

	var paramName string
	switch target.Kind() {
	case "selector_expression":
		operand := target.ChildByFieldName("operand")
		if operand == nil {
			return
		}
		base := selectorBaseIdentifier(operand)
		if base == nil {
			return
		}
		paramName = base.Utf8Text(source)
	case "index_expression":
		operand := target.ChildByFieldName("operand")
		if operand == nil || operand.Kind() != "identifier" {
			return
		}
		paramName = operand.Utf8Text(source)
	default:
		// Plain identifier targets (cfg = other) rebind the local
		// parameter variable rather than writing through it, and any
		// other target kind is out of scope for this detector.
		return
	}

	if !mutableParams[paramName] {
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
// the identifier it ultimately reads: either the operand itself, when it is
// already a plain identifier (cfg.Name), or the identifier wrapped by a
// dereference, when the operand is a parenthesized unary "*" expression
// ((*cfg).Name). Any other operand shape (e.g. a nested selector like
// a.b.Name) is not resolved to a single base identifier and returns nil.
func selectorBaseIdentifier(operand engine.Node) engine.Node {
	if operand.Kind() == "identifier" {
		return operand
	}
	if operand.Kind() != "parenthesized_expression" {
		return nil
	}
	count := operand.ChildCount()
	for i := 0; i < count; i++ {
		child := operand.Child(i)
		if child.Kind() != "unary_expression" {
			continue
		}
		inner := child.ChildByFieldName("operand")
		if inner != nil && inner.Kind() == "identifier" {
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
