package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

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
