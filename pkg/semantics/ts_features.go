package semantics

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// computeTSFeatures walks root exactly once, producing both the structural
// metrics and the tight_coupling findings for a TypeScript or TSX file
// (shared by both languageSpec entries: the walk matches on Node.Kind()
// strings alone, which every Node resolves against its own tree's
// language, so it needs no grammar-specific Query the way import
// extraction does). TypeSwitches and Selects have no TypeScript analog and
// are always 0 (D2).
func computeTSFeatures(root *tree_sitter.Node, source []byte) (StructuralMetrics, []Finding) {
	c := &tsFeatureCollector{}
	c.walk(root, source, 0, false, false)
	return c.metrics, c.findings
}

// tsFeatureCollector accumulates StructuralMetrics and Findings during a
// single pre-order walk of a TS/TSX tree.
type tsFeatureCollector struct {
	metrics  StructuralMetrics
	findings []Finding
}

// tsFunctionLikeKinds is the D2a "function-like-but-not-method" set:
// standalone/expression functions and arrows. Each one increments
// Functions and opens a function scope for the nesting rule (D2b).
// method_definition (class methods) is handled separately since it
// increments Methods instead and additionally participates in
// tight_coupling detection (D3).
var tsFunctionLikeKinds = map[string]bool{
	"function_declaration":           true,
	"function_expression":            true,
	"arrow_function":                 true,
	"generator_function_declaration": true,
	"generator_function":             true,
}

// walk visits n and its descendants in pre-order, incrementing metrics
// counters for the node kinds D2/D2a track and collecting tight_coupling
// findings (D3), all in the one traversal. blockDepth counts nested
// "statement_block" nodes seen so far within the current function-like
// body (0 outside any body); inFunc reports whether the walk is currently
// inside a function-like node (D2a), since nesting depth (D2b) is only
// measured inside those bodies. Entering any function-like node resets
// blockDepth to 0, so depth is measured per function body rather than
// cumulatively across nested functions -- exactly as Go resets on each
// function/method declaration.
//
// inCtorBody reports whether the walk is currently inside a constructor
// method's body: true only between entering a constructor method_definition
// and leaving its subtree. It is reset (to false, or to a fresh
// isConstructorMethod check) on every method_definition and on every
// non-arrow function-like node, since those introduce their own `this`
// binding -- so a nested class's constructor is scanned exactly once, by
// its own method_definition visit, not also by an enclosing constructor's
// scan, and a plain function nested inside a constructor does not
// misattribute its own `this.x = new Y()` to the enclosing constructor.
// Arrow functions do not rebind `this`, so descending into one preserves
// the enclosing inCtorBody value.
func (c *tsFeatureCollector) walk(n *tree_sitter.Node, source []byte, blockDepth int, inFunc bool, inCtorBody bool) {
	if n == nil {
		return
	}

	switch {
	case n.Kind() == "if_statement":
		c.metrics.Ifs++
	case n.Kind() == "for_statement", n.Kind() == "for_in_statement":
		c.metrics.Fors++
	case n.Kind() == "switch_statement":
		c.metrics.ExprSwitches++
	case n.Kind() == "method_definition":
		c.metrics.Methods++
		inFunc = true
		blockDepth = 0
		inCtorBody = isConstructorMethod(n, source)
	case tsFunctionLikeKinds[n.Kind()]:
		c.metrics.Functions++
		inFunc = true
		blockDepth = 0
		if n.Kind() != "arrow_function" {
			inCtorBody = false
		}
	case n.Kind() == "statement_block":
		if inFunc {
			blockDepth++
			if blockDepth > c.metrics.MaxNestingDepth {
				c.metrics.MaxNestingDepth = blockDepth
			}
		}
	case inCtorBody && n.Kind() == "assignment_expression":
		c.checkTightCouplingAssignment(n, source)
	}

	count := n.ChildCount()
	for i := uint(0); i < count; i++ {
		c.walk(n.Child(i), source, blockDepth, inFunc, inCtorBody)
	}
}

// isConstructorMethod reports whether method is a constructor: a
// method_definition whose name field is a property_identifier with source
// text exactly "constructor".
func isConstructorMethod(method *tree_sitter.Node, source []byte) bool {
	nameNode := method.ChildByFieldName("name")
	return nameNode != nil && nameNode.Kind() == "property_identifier" && nameNode.Utf8Text(source) == "constructor"
}

// checkTightCouplingAssignment emits a "tight_coupling" Finding (D3) if n
// (an assignment_expression within a constructor body) assigns a
// new_expression to a property of `this` -- `this.<prop> = new Y()` (a
// member_expression) or `this[<expr>] = new Y()` (a subscript_expression),
// each with an "object" field of kind "this". Assignments to a plain
// variable (`x = new Y()`) or to some other object's property
// (`other.x = new Y()`) are not tight coupling to the constructor's own
// instance and are excluded. variable_declarator initializers
// (`const c = new X()`) are never passed here (only assignment_expression
// nodes trigger this call).
func (c *tsFeatureCollector) checkTightCouplingAssignment(n *tree_sitter.Node, source []byte) {
	left := n.ChildByFieldName("left")
	if left == nil || (left.Kind() != "member_expression" && left.Kind() != "subscript_expression") {
		return
	}
	if object := left.ChildByFieldName("object"); object == nil || object.Kind() != "this" {
		return
	}

	right := n.ChildByFieldName("right")
	if right == nil || right.Kind() != "new_expression" {
		return
	}
	name := ""
	if ctor := right.ChildByFieldName("constructor"); ctor != nil {
		name = ctor.Utf8Text(source)
	}
	c.findings = append(c.findings, Finding{
		Kind:     "tight_coupling",
		Name:     name,
		Location: locationFromNode(right),
	})
}
