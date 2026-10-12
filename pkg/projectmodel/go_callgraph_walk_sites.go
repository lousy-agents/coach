package projectmodel

import (
	"go/types"

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

func (w *callGraphWalk) applyClassification(result callSiteClassification) {
	if result.Fact != nil {
		w.facts = append(w.facts, *result.Fact)
	}
	for _, d := range result.Diagnostics {
		w.diagnostics = append(w.diagnostics, d)
		w.counts[callSiteDiagnosticCounts[d.Code]]++
		if d.Code == DiagCallUnresolvedSyntheticWrapper {
			// classifyCallSite only emits this diagnostic when the
			// wrapper's real target is local, so unlike the other
			// unresolved-call-site classes this always means a real
			// local-to-local call edge was structurally present but
			// never walked; mark the result incomplete rather than
			// merely diagnosed.
			w.complete = false
		}
	}
}
