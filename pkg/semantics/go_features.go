package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

	// goToctouActSeen dedupes toctou_check_then_act Findings by the act
	// call's own source span (see checkGoTOCTOUCheckThenAct), since a
	// nested Stat-gated if on the same path resolves the same act call
	// once per enclosing guard.
	goToctouActSeen map[goToctouLocationKey]bool
	mutatesSeen     map[mutatesInputKey]bool
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
