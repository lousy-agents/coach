package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// reactExtractWorkspaceBranches collects ReactWorkspaceBranch entries from
// two disjoint constructs: discriminant ternary/if-else-if chains gated at
// >=3 JSX-bearing branches per chain, and role="tabpanel" JSX elements
// (ungated). Results are de-duplicated by the branch's own Location.StartByte
// and ordered by that same start_byte.
func reactExtractWorkspaceBranches(body engine.Node, source []byte) []ReactWorkspaceBranch {
	var branches []ReactWorkspaceBranch
	seen := map[uint]struct{}{}
	addBranch := func(b ReactWorkspaceBranch) {
		if _, dup := seen[b.Location.StartByte]; dup {
			return
		}
		seen[b.Location.StartByte] = struct{}{}
		branches = append(branches, b)
	}

	reactCollectDiscriminantBranches(body, source, addBranch)
	reactCollectTabpanelBranches(body, source, addBranch)

	sort.SliceStable(branches, func(i, j int) bool {
		return branches[i].Location.StartByte < branches[j].Location.StartByte
	})
	return branches
}

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

// reactCollectDiscriminantChain walks n forward through same-base chained
// ternary/if-else-if branches and returns the JSX-bearing branches found
// plus every chain node visited (regardless of JSX-bearing outcome), so the
// caller can mark the whole chain consumed even when the gate fails it.
func reactCollectDiscriminantChain(n engine.Node, base string, source []byte) ([]ReactWorkspaceBranch, []engine.Node) {
	switch n.Kind() {
	case "ternary_expression":
		return reactCollectTernaryChain(n, base, source)
	case "if_statement":
		return reactCollectIfChain(n, base, source)
	default:
		return nil, nil
	}
}
