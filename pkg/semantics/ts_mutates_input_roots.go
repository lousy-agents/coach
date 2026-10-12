package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// tsMutationBase resolves expr (a candidate mutation target/argument) down
// to the root identifier it is ultimately rooted at, when expr is a
// member_expression or subscript_expression -- either directly (`p.x`,
// `p[...]`) or through a chain of nested member_expression/
// subscript_expression "object" fields (`p.x.y`, `p.items[0].name`) -- or
// nil for any other shape, including a bare identifier (handled
// separately, since a bare identifier as an assignment's left-hand side is
// a rebind, not a write-through) and a chain that bottoms out in something
// other than a plain identifier (e.g. `f().x`), which is not resolved to a
// root.
func tsMutationBase(expr engine.Node) engine.Node {
	if expr == nil {
		return nil
	}
	if expr.Kind() != "member_expression" && expr.Kind() != "subscript_expression" {
		return nil
	}
	return tsResolveRootIdentifier(expr.ChildByFieldName("object"))
}

// tsResolveRootIdentifier walks a chain of nested member_expression/
// subscript_expression "object" fields, starting at expr, until it reaches
// a plain identifier -- the root -- or determines there is no such root
// (e.g. the chain bottoms out in a call_expression like `f().x`), in which
// case it returns nil. Used by both tsMutationBase (assignment/delete
// targets) and checkMutatesInputCall (method-call receivers) so nested
// mutation targets/receivers rooted at a tracked parameter (`p.x.y = 1`,
// `p.items.push(1)`) are resolved the same way.
func tsResolveRootIdentifier(expr engine.Node) engine.Node {
	for expr != nil {
		switch expr.Kind() {
		case "identifier":
			return expr
		case "member_expression", "subscript_expression":
			expr = expr.ChildByFieldName("object")
		case "parenthesized_expression", "non_null_expression":
			expr = tsWrappedExpressionInner(expr)
		default:
			return nil
		}
	}
	return nil
}
