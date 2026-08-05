package semantics

import (
	"strconv"

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
func (c *tsFeatureCollector) checkMutatesInputAssignment(n engine.Node, source []byte, scopes []tsParamScope) {
	left := n.ChildByFieldName("left")
	base := tsMutationBase(left)
	if base != nil {
		c.recordMutatesInput(base, left, source, scopes)
		return
	}
	c.recordMutatesInputAssignmentTargets(left, source, scopes)
}

// checkMutatesInputDelete emits a "mutates_input" Finding (Story 2) if n
// (a unary_expression) is a `delete` of a property or index rooted at some
// enclosing scope's identifier-bound parameter (`delete p.x`,
// `delete p['x']`). Unlike checkMutatesInputAssignment/checkMutatesInputCall,
// Evidence/Location are taken from n itself (the whole "delete ..."
// expression) rather than just the target: a delete unary_expression has no
// extra unbounded content beyond its "delete" keyword and target argument,
// so it is already short and bounded, and keeping the keyword makes the
// evidence self-explanatory as a deletion rather than a read.
func (c *tsFeatureCollector) checkMutatesInputDelete(n engine.Node, source []byte, scopes []tsParamScope) {
	operator := n.ChildByFieldName("operator")
	if operator == nil || operator.Utf8Text(source) != "delete" {
		return
	}
	base := tsMutationBase(n.ChildByFieldName("argument"))
	if base == nil {
		return
	}
	c.recordMutatesInput(base, n, source, scopes)
}

func (c *tsFeatureCollector) checkMutatesInputUpdate(n engine.Node, source []byte, scopes []tsParamScope) {
	target := n.ChildByFieldName("argument")
	base := tsMutationBase(target)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, target, source, scopes)
}

func (c *tsFeatureCollector) recordMutatesInputAssignmentTargets(n engine.Node, source []byte, scopes []tsParamScope) {
	if n == nil {
		return
	}
	if base := tsMutationBase(n); base != nil {
		c.recordMutatesInput(base, n, source, scopes)
		return
	}
	switch n.Kind() {
	case "parenthesized_expression":
		c.recordMutatesInputAssignmentTargets(tsWrappedExpressionInner(n), source, scopes)
	case "pair_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("value"), source, scopes)
	case "assignment_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("left"), source, scopes)
	case "rest_pattern":
		c.recordMutatesInputAssignmentTargets(n.ChildByFieldName("argument"), source, scopes)
	case "object_pattern", "array_pattern":
		count := n.ChildCount()
		for i := 0; i < count; i++ {
			c.recordMutatesInputAssignmentTargets(n.Child(i), source, scopes)
		}
	}
}

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
func (c *tsFeatureCollector) checkMutatesInputCall(n engine.Node, source []byte, scopes []tsParamScope) {
	fn := n.ChildByFieldName("function")
	if fn == nil {
		return
	}
	object, methodName := tsMutatingMethodReceiver(fn, source)
	if object == nil || !mutatingTSMethodNames[methodName] {
		return
	}
	base := tsResolveRootIdentifier(object)
	if base == nil {
		return
	}
	c.recordMutatesInput(base, fn, source, scopes)
}

func tsMutatingMethodReceiver(fn engine.Node, source []byte) (engine.Node, string) {
	switch fn.Kind() {
	case "member_expression":
		property := fn.ChildByFieldName("property")
		if property == nil {
			return nil, ""
		}
		return fn.ChildByFieldName("object"), property.Utf8Text(source)
	case "subscript_expression":
		index := fn.ChildByFieldName("index")
		methodName, ok := tsStringLiteralText(index, source)
		if !ok {
			return nil, ""
		}
		return fn.ChildByFieldName("object"), methodName
	default:
		return nil, ""
	}
}

func tsStringLiteralText(n engine.Node, source []byte) (string, bool) {
	if n == nil || n.Kind() != "string" {
		return "", false
	}
	value, err := strconv.Unquote(n.Utf8Text(source))
	if err != nil {
		return "", false
	}
	return value, true
}

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
