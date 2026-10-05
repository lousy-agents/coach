package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactDiscriminantOps is the set of equality operators a workspace-branch
// discriminant condition may use.
var reactDiscriminantOps = map[string]bool{"===": true, "!==": true, "==": true, "!=": true}

// reactExtractWorkspaceBranches collects ReactWorkspaceBranch entries from
// two disjoint constructs: discriminant ternary/if-else-if chains gated at
// >=3 JSX-bearing branches per chain, and role="tabpanel" JSX elements
// (ungated). Results are de-duplicated by the branch's own Location.StartByte
// and ordered by that same start_byte.

func reactCollectDiscriminantBranches(body engine.Node, source []byte, add func(ReactWorkspaceBranch)) {
	consumed := map[[2]uint]struct{}{}
	reactWalkScope(body, source, func(n engine.Node) {
		switch n.Kind() {
		case "ternary_expression", "if_statement":
		default:
			return
		}
		key := [2]uint{n.StartByte(), n.EndByte()}
		if _, done := consumed[key]; done {
			return
		}
		base, ok := reactDiscriminantConditionBase(n, source)
		if !ok {
			return
		}
		chainBranches, chainNodes := reactCollectDiscriminantChain(n, base, source)
		for _, cn := range chainNodes {
			consumed[[2]uint{cn.StartByte(), cn.EndByte()}] = struct{}{}
		}
		if len(chainBranches) < 3 {
			return
		}
		for _, b := range chainBranches {
			add(b)
		}
	})
}

// reactDiscriminantConditionBase reports whether n's "condition" field is a
// binary_expression testing equality/inequality between a discriminant base
// (a plain identifier, or a member_expression -- whose full text, not just
// its property, is used as the base so structurally distinct discriminants
// like props.v and other.v never compare equal) and a string/number
// literal, and returns that base text.
func reactDiscriminantConditionBase(n engine.Node, source []byte) (string, bool) {
	cond := unwrapTSParen(n.ChildByFieldName("condition"))
	if cond == nil || cond.Kind() != "binary_expression" {
		return "", false
	}
	if !reactDiscriminantOps[tsBinaryOp(cond)] {
		return "", false
	}
	left := cond.ChildByFieldName("left")
	right := cond.ChildByFieldName("right")
	if left == nil || right == nil {
		return "", false
	}
	if base, ok := reactDiscriminantBase(left, right, source); ok {
		return base, true
	}
	return reactDiscriminantBase(right, left, source)
}

// reactCollectDiscriminantChain walks n forward through same-base chained
// ternary/if-else-if branches and returns the JSX-bearing branches found
// plus every chain node visited (regardless of JSX-bearing outcome), so the
// caller can mark the whole chain consumed even when the gate fails it.

// Terminal residual alternative of the same-base chain (e.g. the
// final `: <DefaultPanel />` arm). null/undefined/non-JSX yield no branch.

// reactIfChainContinue returns the next if_statement in an else-if chain, or
// a terminal residual branch when the chain ends in a final else body.

// reactResidualBranch emits a workspace branch for a chain's terminal
// residual alternative / final else body when it contains JSX. Label uses
// Design precedence steps 2–3 only (no equality literal on residual arms).

// reactWorkspaceBranchLabel applies the label precedence rule: the
// discriminant condition's literal text first, else a capitalized primary
// JSX child's tag name, else the "<branch>" sentinel.

// reactTabpanelBranch reports whether n is a JSX opening/self-closing
// element carrying role="tabpanel", and if so returns its branch: label is
// its aria-label attribute's string value, else its id attribute's string
// value, else the literal "tabpanel".
