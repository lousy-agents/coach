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
	if nested {
		if kind == paramNotMutable {
			return
		}
	} else if kind != requiredKind {
		return
	}
	c.recordMutatesInput(funcName, paramName, target, source)
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
