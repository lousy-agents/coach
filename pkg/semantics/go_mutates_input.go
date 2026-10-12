package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// mutatesInputKey dedupes findings by (parameter, mutation-expression
// location) per AC-5: repeated writes to the same parameter through the
// same source location must not produce duplicate findings.
type mutatesInputKey struct {
	paramName string
	startByte uint
	endByte   uint
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

	c.mutatesSeen = map[mutatesInputKey]bool{}
	c.findAssignments(body, source, funcName, mutableParams)
}

// findAssignments walks n's subtree looking for assignment_statement nodes
// whose left-hand-side targets write through a mutable parameter, and
// records a deduplicated "mutates_input" Finding for each. It does not
// descend specially into nested function_declaration/func_literal bodies:
// a closure's own mutation of an outer mutable parameter is still a
// caller-visible mutation of that parameter and is deliberately still
// reported.
func (c *featureCollector) findAssignments(n engine.Node, source []byte, funcName string, mutableParams map[string]paramMutKind) {
	if n == nil {
		return
	}

	if n.Kind() == "type_switch_statement" {
		// The type-switch alias (e.g. `cfg` in `switch cfg := v.(type)`) is
		// scoped only to this switch statement's own subtree, so it must
		// shadow mutableParams for the recursive descent below but must not
		// leak into the shadowing applied to the enclosing block's later
		// siblings at the bottom of this function.
		mutableParams = shadowNames(mutableParams, identifiersInNodeField(n, "alias", source))
	}

	if n.Kind() == "assignment_statement" {
		c.checkAssignmentTargets(n.ChildByFieldName("left"), source, funcName, mutableParams)
	}
	if n.Kind() == "inc_statement" || n.Kind() == "dec_statement" {
		c.checkAssignmentTarget(updateStatementTarget(n), source, funcName, mutableParams)
	}
	if n.Kind() == "func_literal" {
		if params := n.ChildByFieldName("parameters"); params != nil {
			mutableParams = shadowParamTypes(mutableParams, params, source)
		}
	}
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		child := n.Child(i)
		c.findAssignments(child, source, funcName, mutableParams)
		mutableParams = shadowLocalDeclarations(mutableParams, child, source)
	}
}

func (c *featureCollector) recordMutatesInput(funcName, paramName string, target engine.Node, source []byte) {
	loc := locationFromNode(target)
	key := mutatesInputKey{paramName: paramName, startByte: loc.StartByte, endByte: loc.EndByte}
	if c.mutatesSeen[key] {
		return
	}
	c.mutatesSeen[key] = true
	c.findings = append(c.findings, newMutatesInputFinding(funcName, paramName, target, source))
}
