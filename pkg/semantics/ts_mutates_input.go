package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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

// checkMutatesInputForNode runs the mutates_input detector (Story 2)
// matching n's own kind, when scopes has at least one enclosing
// function-like/method scope to attribute a mutation to.
func (c *tsFeatureCollector) checkMutatesInputForNode(n engine.Node, source []byte, scopes []tsParamScope) {
	if len(scopes) == 0 {
		return
	}
	switch n.Kind() {
	case "assignment_expression", "augmented_assignment_expression":
		c.checkMutatesInputAssignment(n, source, scopes)
	case "unary_expression":
		c.checkMutatesInputDelete(n, source, scopes)
	case "call_expression":
		c.checkMutatesInputCall(n, source, scopes)
	case "update_expression":
		c.checkMutatesInputUpdate(n, source, scopes)
	}
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
