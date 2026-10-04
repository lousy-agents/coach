package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
		c.checkGoTOCTOUCheckThenAct(n, source)
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
func (s identSet) collectParams(n engine.Node, source []byte) {
	if n == nil {
		return
	}
	switch n.Kind() {
	case "parameter_declaration", "variadic_parameter_declaration":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			child := n.Child(i)
			if child.Kind() == "identifier" {
				s[child.Utf8Text(source)] = true
			}
		}
		return
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		s.collectParams(n.Child(i), source)
	}
}
