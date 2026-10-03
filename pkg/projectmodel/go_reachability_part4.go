package projectmodel

import (
	"context"

	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// httpHandlerFuncSignature looks up net/http.HandlerFunc's underlying
// *types.Signature from prog or the initial packages' type-checker import
// graph, returning nil if net/http was not part of this root's build.
func httpHandlerFuncSignature(prog *ssa.Program, pkgs []*packages.Package) *types.Signature {
	tp := typesPackageByPath(prog, pkgs, "net/http")
	if tp == nil {
		return nil
	}
	obj := tp.Scope().Lookup("HandlerFunc")
	if obj == nil {
		return nil
	}
	sig, ok := obj.Type().Underlying().(*types.Signature)
	if !ok {
		return nil
	}
	return sig
}

// buildCallGraphAdjacency renders facts as a sorted, deduplicated adjacency
// map so bfsShortestPaths' tie-breaking never depends on facts' input
// order or Go map iteration order.
func buildCallGraphAdjacency(facts []CallFact) map[string][]string {
	tmp := map[string]map[string]bool{}
	for _, f := range facts {
		if tmp[f.From] == nil {
			tmp[f.From] = map[string]bool{}
		}
		tmp[f.From][f.To] = true
	}
	adjacency := make(map[string][]string, len(tmp))
	for from, tos := range tmp {
		adjacency[from] = mapKeysSorted(tos)
	}
	return adjacency
}

// findGoReachabilitySourcesFromLoaded walks loaded's local functions and
// returns every function whose signature is identical to
// net/http.HandlerFunc's underlying func(http.ResponseWriter, *http.Request).
// Source identification needs each function's own signature, which
// CallGraphResult's From/To strings do not carry, so this walk stays
// separate from the call-graph walk rather than growing CallFact.
func findGoReachabilitySourcesFromLoaded(ctx context.Context, loaded *loadedGoSnapshot) ([]string, bool, []Diagnostic) {
	if ctx.Err() != nil {
		return nil, false, []Diagnostic{{Code: DiagReachabilityBudgetExceeded}}
	}

	search := goReachabilitySourceSearch{
		seen:     map[string]bool{},
		complete: loaded.discovery.Complete,
	}
	for _, root := range loaded.roots {
		if search.collectRoot(ctx, loaded, root) {
			break
		}
	}
	return mapKeysSorted(search.seen), search.complete, search.diagnostics
}
