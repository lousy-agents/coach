package projectmodel

import (
	"sort"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// sortedLocalFunctions returns every function with a body (fn.Blocks != nil)
// belonging to a package in localPkgPaths, sorted by RelString(nil) so
// budget truncation cuts the same trailing set on every call regardless of
// ssa/go-types' internal map iteration order.
func sortedLocalFunctions(prog *ssa.Program, localPkgPaths map[string]bool) []*ssa.Function {
	all := ssautil.AllFunctions(prog)
	out := make([]*ssa.Function, 0, len(all))
	for fn := range all {
		if fn.Blocks == nil || fn.Pkg == nil {
			continue
		}
		if !localPkgPaths[fn.Pkg.Pkg.Path()] {
			continue
		}
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelString(nil) < out[j].RelString(nil) })
	return out
}
