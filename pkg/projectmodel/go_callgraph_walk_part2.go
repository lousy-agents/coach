package projectmodel

import (
	"context"
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

func (w *callGraphWalk) walkFunction(fn *ssa.Function, root loadedGoRoot, loaded *loadedGoSnapshot, opts CallGraphOptions, httpHandlerIface *types.Interface) (stop bool) {
	for _, blk := range fn.Blocks {
		if w.walkBlock(fn, blk, root, loaded, opts, httpHandlerIface) {
			return true
		}
	}
	return false
}

func (w *callGraphWalk) walkBlock(fn *ssa.Function, blk *ssa.BasicBlock, root loadedGoRoot, loaded *loadedGoSnapshot, opts CallGraphOptions, httpHandlerIface *types.Interface) (stop bool) {
	for _, instr := range blk.Instrs {
		site, ok := instr.(ssa.CallInstruction)
		if !ok {
			continue
		}
		if opts.Budgets.MaxGraphEdges > 0 && w.edgesProcessed >= opts.Budgets.MaxGraphEdges {
			w.noteBudgetExceeded(root.dir)
			return true
		}
		w.edgesProcessed++
		w.counts["call_sites_seen"]++
		w.applyClassification(classifyCallSite(fn, site, loaded.tempDir, httpHandlerIface, root.localPkgPaths))
	}
	return false
}
func (w *callGraphWalk) walkRoots(ctx context.Context, loaded *loadedGoSnapshot, opts CallGraphOptions) {
	for _, root := range loaded.roots {
		if ctx.Err() != nil {
			w.noteBudgetExceeded(root.dir)
			return
		}
		if w.walkRoot(ctx, root, loaded, opts) {
			return
		}
	}
}

// httpHandlerInterface looks up net/http.Handler's interface type from
// prog or the initial packages' type-checker import graph, returning nil
// if net/http was not part of this root's build (so isFunctionValueArg
// falls back to its func-typed check only).
func httpHandlerInterface(prog *ssa.Program, pkgs []*packages.Package) *types.Interface {
	tp := typesPackageByPath(prog, pkgs, "net/http")
	if tp == nil {
		return nil
	}
	obj := tp.Scope().Lookup("Handler")
	if obj == nil {
		return nil
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	return iface
}
