package projectmodel

import (
	"context"
)

// LayerBypassOptions bounds one BuildGoLayerBypass call. Roots and Budgets
// are forwarded to the underlying BuildGoCallGraph call. MaxSearchNodes
// additionally bounds the total number of call-graph nodes visited across
// every source's BFS traversal combined; zero means unbounded.
type LayerBypassOptions struct {
	RequiredLayer  BypassLayer
	Roots          []string
	Budgets        GoBudgets
	MaxSearchNodes int
}

// BuildGoLayerBypass finds layer-bypass witnesses: a source/sink pair (the
// same handler-shaped source and pinned database-sink registry
// BuildGoReachability uses) with a statically resolved CallFact path that
// survives deleting every call-graph node whose owning package directory
// falls under opts.RequiredLayer's prefixes. A surviving path is, by
// construction, a route from source to sink that never passes through the
// required layer -- the witness IS the proof, and it is emitted whenever
// step finds one, regardless of whether a separate compliant path (one that
// does pass through the required layer) also exists elsewhere in the graph.
//
// A witness is only ever emitted at LayerBypassConfidenceHigh, and only when
// opts.RequiredLayer is unambiguous (its Prefixes are non-empty and match at
// least one local package in the snapshot -- see
// DiagLayerBypassAmbiguousLayer) and the underlying call graph, source
// identification, and node-classification walk all completed within budget
// for the pair involved. Any of those conditions failing suppresses the
// witness for that pair entirely rather than downgrading its confidence;
// see LayerBypassResult.Coverage for what was and was not evaluated.
//
// BuildGoLayerBypass never returns a non-nil error for a per-root load
// failure or budget/context exhaustion; those are reported through
// Coverage.Diagnostics/Coverage.Complete, matching BuildGoReachability's
// fail-open-with-diagnostics contract.

// ambiguousLayer guards against the false-positive case where an
// unconfigured/unmatched RequiredLayer would remove nothing from
// adjacency, silently turning this into an ordinary reachability search
// that could misreport a genuinely compliant path as a bypass witness.

// layerBypassSearch is the result of one source/sink witness search.
// truncatedSearch reflects this search's own budget (MaxSearchNodes, ctx
// wall-time/cancellation) plus the one non-budget case, ambiguousLayer,
// where the search never runs at all. It does not reflect
// callGraphIncomplete or !dirsComplete: both already carry their own
// diagnostic and already force complete to false, and both already force
// every pair to skip as truncated. Folding either into truncatedSearch
// would additionally claim a project_layer_bypass_budget_exceeded that
// never happened -- e.g. a call-graph dead end at a local-targeted
// synthetic wrapper, or a root load failure.
type layerBypassSearch struct {
	witnesses            []LayerBypassWitness
	evaluated            int
	truncatedPairs       int
	nodesVisited         int
	truncatedSearch      bool
	unclassifiedNodeSeen bool
}

// layerBypassContainsDir mirrors codesignal's layerContainsDir: "." matches
// every directory, otherwise a prefix matches dir itself or any "/"-
// separated descendant of it.

// stepPathFullyClassified reports whether every non-sink node on stepPath
// (i.e. every node but the last) has an entry in nodePositions. A node with
// no entry has an unresolvable package directory (see fnPosition), so its
// required-layer membership was never evaluated; the caller must treat that
// as ambiguous rather than assume the node is outside RequiredLayer.
func stepPathFullyClassified(stepPath []ReachabilityStep, nodePositions map[string]layerBypassNodePosition) bool {
	if len(stepPath) == 0 {
		return true
	}
	for _, step := range stepPath[:len(stepPath)-1] {
		if _, ok := nodePositions[step.NodeID]; !ok {
			return false
		}
	}
	return true
}

// layerBypassSteps converts stepPath (reconstructReachabilityPath's shared
// ReachabilityStep shape) into LayerBypassSteps, filling Path/Line from
// nodePositions for whichever nodes resolved a position -- leaving them
// zero-valued for the rest (typically only the sink).

// removeLayerNodesFromAdjacency returns adjacency with every node in
// removed deleted, both as an edge source and as an edge destination, so a
// BFS over the result can only find a path that never touches one of those
// nodes. Surviving neighbor lists stay in the same sorted order adjacency
// already used, preserving bfsShortestPaths' deterministic tie-breaking.

// layerBypassNodePosition is one local function's resolved declaration
// position: Dir drives RequiredLayer classification (see
// layerBypassContainsDir), File/Line are the repository-relative position
// LayerBypassStep.Path/Line carry into LayerBypassWitness.Path.
type layerBypassNodePosition struct {
	Dir  string
	File string
	Line int
}

// layerBypassNodePackageDirsFromLoaded walks loaded's local functions,
// returning every local function's RelString(nil) identity mapped to its
// resolved declaration position. This walk stays separate from the
// call-graph walk because CallFact's From/To strings carry no directory or
// position information, and separate from source identification because it
// needs every local function, not just handler-shaped ones.
func layerBypassNodePackageDirsFromLoaded(ctx context.Context, loaded *loadedGoSnapshot) (map[string]layerBypassNodePosition, bool, []Diagnostic) {
	if ctx.Err() != nil {
		return nil, false, []Diagnostic{{Code: DiagLayerBypassBudgetExceeded}}
	}

	acc := &layerBypassPositionAcc{
		positions: map[string]layerBypassNodePosition{},
		complete:  loaded.discovery.Complete,
		tempDir:   loaded.tempDir,
	}

	for _, root := range loaded.roots {
		if ctx.Err() != nil {
			acc.complete = false
			acc.diagnostics = append(acc.diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded, Path: root.dir})
			break
		}
		if root.loadErr != nil {
			acc.complete = false
			acc.diagnostics = append(acc.diagnostics, Diagnostic{Code: DiagLayerBypassSourceLoadFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
			continue
		}
		acc.addFunctions(ctx, root)
	}

	return acc.positions, acc.complete, acc.diagnostics
}

type layerBypassPositionAcc struct {
	positions   map[string]layerBypassNodePosition
	diagnostics []Diagnostic
	complete    bool
	tempDir     string
}

func (a *layerBypassPositionAcc) addFunctions(ctx context.Context, root loadedGoRoot) {
	for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
		if ctx.Err() != nil {
			a.complete = false
			a.diagnostics = append(a.diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded, Path: root.dir})
			return
		}
		pos, ok := fnPosition(a.tempDir, fn)
		if !ok {
			continue
		}
		a.positions[fn.RelString(nil)] = pos
	}
}

// fnPosition resolves fn's declaration position to a repository-relative
// package directory, file, and 1-based line, stripping materializeSnapshot's
// absolute tempDir prefix the same way relCallSitePath does for call sites.
// It reports false for a function with no resolvable position (e.g. a
// synthetic wrapper).

// remapDiagnosticCodes returns a copy of diags with every Code present in
// codes rewritten to its mapped value, leaving any other diagnostic
// untouched. It is used to fold a shared helper's diagnostics (e.g.
// findGoReachabilitySources') into this evaluator's own diagnostic-code
// vocabulary rather than leaking a different feature's codes into
// LayerBypassResult.Coverage.

// effectiveLayerBypassBudgets renders opts as LayerBypassResult.Coverage.Budgets,
// mirroring effectiveReachabilityBudgets exactly: the full
// EffectiveGoBudgets(opts.Budgets) vocabulary plus a "search_nodes" key for
// MaxSearchNodes.
func effectiveLayerBypassBudgets(opts LayerBypassOptions) map[string]int {
	budgets := EffectiveGoBudgets(opts.Budgets)
	budgets["search_nodes"] = opts.MaxSearchNodes
	return budgets
}
