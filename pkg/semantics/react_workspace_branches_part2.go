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

		if b, ok := reactResidualBranch(alt, source); ok {
			branches = append(branches, b)
		}
		break
	}
	return branches, nodes
}
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
