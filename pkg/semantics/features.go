package semantics

import (
	"regexp"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
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
func computeGoFeatures(root *tree_sitter.Node, source []byte) (StructuralMetrics, []Finding) {
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
func (c *featureCollector) walk(n *tree_sitter.Node, source []byte, blockDepth int, inFunc bool) {
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
		inFunc = true
		blockDepth = 0
	case "method_declaration":
		c.metrics.Methods++
		c.checkPointerReturn(n, source)
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
	for i := uint(0); i < count; i++ {
		c.walk(n.Child(i), source, blockDepth, inFunc)
	}
}

// checkConstructorFunc emits a "constructor_func" Finding (AC-3.5) if decl's
// name field matches constructorFuncNameRe.
func (c *featureCollector) checkConstructorFunc(decl *tree_sitter.Node, source []byte) {
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
func (c *featureCollector) checkPointerReturn(decl *tree_sitter.Node, source []byte) {
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
func resultHasPointerType(result *tree_sitter.Node) bool {
	if result.Kind() == "pointer_type" {
		return true
	}
	count := result.ChildCount()
	for i := uint(0); i < count; i++ {
		if resultHasPointerType(result.Child(i)) {
			return true
		}
	}
	return false
}
