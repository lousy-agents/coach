package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactCollectTernaryChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	var branches []ReactWorkspaceBranch
	var nodes []engine.Node
	cur := n
	for cur != nil && cur.Kind() == "ternary_expression" {
		curBase, ok := reactDiscriminantConditionBase(cur, source)
		if !ok || curBase != base {
			break
		}
		nodes = append(nodes, cur)
		if b, ok := reactTernaryBranch(cur, source); ok {
			branches = append(branches, b)
		}
		alt := unwrapTSParen(cur.ChildByFieldName("alternative"))
		if alt != nil && alt.Kind() == "ternary_expression" {
			cur = alt
			continue
		}
		// Terminal residual alternative of the same-base chain (e.g. the
		// final `: <DefaultPanel />` arm). null/undefined/non-JSX yield no branch.
		if b, ok := reactResidualBranch(alt, source); ok {
			branches = append(branches, b)
		}
		break
	}
	return branches, nodes
}

func reactTernaryBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	cons := unwrapTSParen(n.ChildByFieldName("consequence"))
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(n, cons, source),
		Location: locationFromNode(cons),
	}, true
}

// reactResidualBranch emits a workspace branch for a chain's terminal
// residual alternative / final else body when it contains JSX. Label uses
// Design precedence steps 2–3 only (no equality literal on residual arms).
func reactResidualBranch(cons engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(nil, cons, source),
		Location: locationFromNode(cons),
	}, true
}
