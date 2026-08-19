package projectmodel

import (
	"context"
	"go/types"

	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// callGraphWalk is the mutable state of one buildGoCallGraphFromLoaded pass.
// It exists so the per-root / per-function / per-site walks can share facts,
// coverage, and budget counters without a labeled break across four loops.
type callGraphWalk struct {
	facts          []CallFact
	diagnostics    []Diagnostic
	counts         map[string]int
	complete       bool
	truncated      bool
	nodesProcessed int
	edgesProcessed int
}

func newCallGraphWalk(loaded *loadedGoSnapshot) callGraphWalk {
	return callGraphWalk{
		diagnostics: append([]Diagnostic{}, loaded.discovery.Diagnostics...),
		complete:    loaded.discovery.Complete,
		truncated:   loaded.loadStopped,
		counts: map[string]int{
			"roots_seen":                        len(loaded.moduleDirs),
			"roots_built":                       0,
			"functions_seen":                    0,
			"call_sites_seen":                   0,
			"unresolved_interface":              0,
			"unresolved_function_value":         0,
			"unresolved_reflection":             0,
			"unresolved_framework_registration": 0,
			"unresolved_synthetic_wrapper":      0,
			"callgraph_static_nodes":            0,
			"ssa_programs_built":                loaded.programsBuilt(),
		},
	}
}

func (w *callGraphWalk) noteBudgetExceeded(path string) {
	w.truncated = true
	w.diagnostics = append(w.diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: path})
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

func (w *callGraphWalk) walkRoot(ctx context.Context, root loadedGoRoot, loaded *loadedGoSnapshot, opts CallGraphOptions) (stop bool) {
	if root.loadErr != nil {
		w.complete = false
		w.diagnostics = append(w.diagnostics, Diagnostic{Code: DiagCallGraphBuildFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
		return false
	}
	for _, p := range root.pkgs {
		for _, e := range p.Errors {
			w.complete = false
			w.diagnostics = append(w.diagnostics, Diagnostic{Code: DiagCallGraphBuildFailed, Path: root.dir, Message: stripTempDir(e.Error(), loaded.tempDir)})
		}
	}
	w.counts["roots_built"]++

	// CallFacts deliberately come from the sortedLocalFunctions walk
	// below, not from cg.Nodes/cg.Edges: that walk gives a single,
	// deterministically ordered pass that both resolves direct callees
	// and classifies every unresolved call site (interface, function
	// value, reflection, framework registration, synthetic wrapper) in
	// one place. Walking cg's own maps would reintroduce Go map
	// iteration order and drop the unresolved-site diagnostics; cg is
	// kept only for the pinned static-analysis-backend cross-check
	// below.
	cg := static.CallGraph(root.prog)
	w.counts["callgraph_static_nodes"] += len(cg.Nodes)
	httpHandlerIface := httpHandlerInterface(root.prog, root.pkgs)

	for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
		if ctx.Err() != nil {
			w.noteBudgetExceeded(root.dir)
			return true
		}
		if opts.Budgets.MaxGraphNodes > 0 && w.nodesProcessed >= opts.Budgets.MaxGraphNodes {
			w.noteBudgetExceeded(root.dir)
			return true
		}
		w.nodesProcessed++
		w.counts["functions_seen"]++
		if w.walkFunction(fn, root, loaded, opts, httpHandlerIface) {
			return true
		}
	}
	return false
}

func (w *callGraphWalk) walkFunction(fn *ssa.Function, root loadedGoRoot, loaded *loadedGoSnapshot, opts CallGraphOptions, httpHandlerIface *types.Interface) (stop bool) {
	for _, blk := range fn.Blocks {
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
	}
	return false
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
