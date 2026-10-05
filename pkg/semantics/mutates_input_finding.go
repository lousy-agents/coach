package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// newMutatesInputFinding builds a "mutates_input" Finding (Story 1/3) for a
// mutation of parameter paramName within function/method funcName, located
// at evidence's own source span so tooling can point directly at the
// mutating expression rather than the enclosing declaration.
func newMutatesInputFinding(funcName, paramName string, evidence engine.Node, source []byte) Finding {
	return Finding{
		Kind:           "mutates_input",
		Name:           funcName + ":" + paramName,
		Location:       locationFromNode(evidence),
		Confidence:     "medium",
		Evidence:       evidence.Utf8Text(source),
		Recommendation: "Return a copy instead of mutating the caller's value, or document/rename this function to make the in-place mutation explicit.",
		SuggestedSkill: "refactor-hidden-mutation",
	}
}
