package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func (c *featureCollector) recordMutableSelectorOrIndex(target engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind, requiredKind paramMutKind) {
	operand := target.ChildByFieldName("operand")
	if operand == nil {
		return
	}
	base, nested := selectorBaseIdentifier(operand, source)
	if base == nil {
		return
	}
	paramName := base.Utf8Text(source)
	kind := mutableParams[paramName]
	if nested {
		if kind == paramNotMutable {
			return
		}
	} else if kind != requiredKind {
		return
	}
	c.recordMutatesInput(funcName, paramName, target, source)
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
