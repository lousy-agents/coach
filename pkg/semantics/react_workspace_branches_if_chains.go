package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func reactCollectIfChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	var branches []ReactWorkspaceBranch
	var nodes []engine.Node
	cur := n
	for cur != nil && cur.Kind() == "if_statement" {
		curBase, ok := reactDiscriminantConditionBase(cur, source)
		if !ok || curBase != base {
			break
		}
		nodes = append(nodes, cur)
		if b, ok := reactIfBranch(cur, source); ok {
			branches = append(branches, b)
		}
		next, terminal := reactIfChainContinue(cur, source)
		if next != nil {
			cur = next
			continue
		}
		if terminal != nil {
			branches = append(branches, *terminal)
		}
		break
	}
	return branches, nodes
}

func reactIfBranch(n engine.Node, source []byte) (ReactWorkspaceBranch, bool) {
	cons := n.ChildByFieldName("consequence")
	if cons == nil || !reactContainsJSX(cons) {
		return ReactWorkspaceBranch{}, false
	}
	return ReactWorkspaceBranch{
		Label:    reactWorkspaceBranchLabel(n, cons, source),
		Location: locationFromNode(cons),
	}, true
}

// reactIfChainContinue returns the next if_statement in an else-if chain, or
// a terminal residual branch when the chain ends in a final else body.
func reactIfChainContinue(cur engine.Node, source []byte) (next engine.Node, terminal *ReactWorkspaceBranch) {
	alt := cur.ChildByFieldName("alternative")
	if alt != nil && alt.Kind() == "else_clause" {
		alt = tsElseClauseInner(alt)
	}
	if alt != nil && alt.Kind() == "if_statement" {
		return alt, nil
	}
	if b, ok := reactResidualBranch(alt, source); ok {
		return nil, &b
	}
	return nil, nil
}
