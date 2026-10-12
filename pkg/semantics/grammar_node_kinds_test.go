package semantics

import (
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// kindSet collects every node kind string it encounters. It stands in for
// go-tree-sitter's ToSexp() (not part of the engine seam gotreesitter's Node
// satisfies): the discovery-gate tests below only need to prove a given node
// kind string exists somewhere in the parsed tree, not reproduce ToSexp()'s
// exact textual format.
type kindSet map[string]bool

func (kinds kindSet) collect(n engine.Node) {
	if n == nil {
		return
	}
	kinds[n.Kind()] = true
	for i := 0; i < n.ChildCount(); i++ {
		kinds.collect(n.Child(i))
	}
}

func dumpKinds(kinds kindSet) string {
	all := make([]string, 0, len(kinds))
	for k := range kinds {
		all = append(all, k)
	}
	sort.Strings(all)
	return strings.Join(all, ", ")
}
