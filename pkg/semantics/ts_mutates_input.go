package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// mutatingTSMethodNames is the exact set of built-in Array/Map/Set method
// names whose call on a parameter-rooted receiver is treated as an
// in-place mutation of that parameter (Story 2). Arbitrary custom methods
// (e.g. `user.setName()`) are deliberately not in this set and so are
// never flagged.
var mutatingTSMethodNames = map[string]bool{
	"copyWithin": true,
	"fill":       true,
	"pop":        true,
	"push":       true,
	"reverse":    true,
	"shift":      true,
	"sort":       true,
	"splice":     true,
	"unshift":    true,
	"set":        true,
	"add":        true,
	"delete":     true,
	"clear":      true,
}

// tsMutatesInputKey dedupes mutates_input findings by (owning function,
// parameter, mutation-expression location), mirroring the Go detector's
// dedup rule: repeated mutation of the same parameter through the same
// source location must not produce duplicate findings.
type tsMutatesInputKey struct {
	ownerName string
	paramName string
	startByte uint
	endByte   uint
}

// checkMutatesInputAssignment emits a "mutates_input" Finding (Story 2) if
// n's left-hand side writes through a property or index rooted at some
// enclosing scope's identifier-bound parameter -- `p.x = ...`
// (member_expression) or `p[...] = ...` (subscript_expression), each with
// an "object" field resolving to a tracked parameter identifier. A plain
// identifier left-hand side (`p = other`) rebinds the local parameter
// variable rather than writing through it and is deliberately excluded.
// Evidence/Location are taken from the target (left-hand side) alone, not
// the whole assignment_expression: an assignment's right-hand side can be
// arbitrarily long (`p.x = someVeryLargeExpression()`), which would
// conflict with Evidence staying short, and would also diverge from the Go
// detector, whose evidence is likewise just the mutated selector/index
// target (e.g. cfg.Name), never including the assigned value.

// checkMutatesInputDelete emits a "mutates_input" Finding (Story 2) if n
// (a unary_expression) is a `delete` of a property or index rooted at some
// enclosing scope's identifier-bound parameter (`delete p.x`,
// `delete p['x']`). Unlike checkMutatesInputAssignment/checkMutatesInputCall,
// Evidence/Location are taken from n itself (the whole "delete ..."
// expression) rather than just the target: a delete unary_expression has no
// extra unbounded content beyond its "delete" keyword and target argument,
// so it is already short and bounded, and keeping the keyword makes the
// evidence self-explanatory as a deletion rather than a read.

// checkMutatesInputCall emits a "mutates_input" Finding (Story 2) if n (a
// call_expression) calls one of mutatingTSMethodNames on a receiver
// rooted at some enclosing scope's identifier-bound parameter, either
// directly (`p.push(x)`, `arr.sort()`, `m.set(k, v)`) or through a chain of
// nested member/subscript accesses (`p.items.push(1)`). Arbitrary custom
// method calls (`user.setName()`) are not in mutatingTSMethodNames and so
// never match. Evidence/Location are taken from fn (the receiver.method
// member_expression, e.g. "p.items.push"), not the whole call_expression:
// a call's arguments can be arbitrarily long or complex
// (`p.items.push(someVeryLargeExpression())`), which would conflict with
// Evidence staying short, and would also diverge from the Go detector's
// bounded, target-only evidence.

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

// tsResolveRootIdentifier walks a chain of nested member_expression/
// subscript_expression "object" fields, starting at expr, until it reaches
// a plain identifier -- the root -- or determines there is no such root
// (e.g. the chain bottoms out in a call_expression like `f().x`), in which
// case it returns nil. Used by both tsMutationBase (assignment/delete
// targets) and checkMutatesInputCall (method-call receivers) so nested
// mutation targets/receivers rooted at a tracked parameter (`p.x.y = 1`,
// `p.items.push(1)`) are resolved the same way.

func tsWrappedExpressionInner(expr engine.Node) engine.Node {
	for _, field := range []string{"expression", "operand", "argument"} {
		if child := expr.ChildByFieldName(field); child != nil {
			return child
		}
	}
	count := expr.ChildCount()
	for i := 0; i < count; i++ {
		child := expr.Child(i)
		switch child.Kind() {
		case "(", ")", "!":
			continue
		default:
			return child
		}
	}
	return nil
}

// recordMutatesInput resolves base's identifier name against scopes
// (innermost to outermost, so a nested function's own same-named parameter
// shadows an outer one -- D6) and, if it is a tracked identifier-bound
// parameter of some scope, records a deduplicated "mutates_input" Finding
// attributing the mutation at evidence's own source span to that scope's
// owner name.
func (c *tsFeatureCollector) recordMutatesInput(base engine.Node, evidence engine.Node, source []byte, scopes []tsParamScope) {
	name := base.Utf8Text(source)
	var owner string
	found := false
	for i := len(scopes) - 1; i >= 0; i-- {
		isParam, ok := scopes[i].bindings[name]
		if !ok {
			continue
		}
		if !isParam {
			return
		}
		owner = scopes[i].ownerName
		found = true
		break
	}
	if !found {
		return
	}

	loc := locationFromNode(evidence)
	key := tsMutatesInputKey{ownerName: owner, paramName: name, startByte: loc.StartByte, endByte: loc.EndByte}
	if c.mutatesInputSeen == nil {
		c.mutatesInputSeen = map[tsMutatesInputKey]bool{}
	}
	if c.mutatesInputSeen[key] {
		return
	}
	c.mutatesInputSeen[key] = true

	c.findings = append(c.findings, newMutatesInputFinding(owner, name, evidence, source))
}
