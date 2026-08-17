package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"golang.org/x/tools/go/ssa"
)

// LayerBypassAlgorithm identifies the pinned deterministic layer-bypass
// traversal BuildGoLayerBypass uses: the same source/sink registry and
// CallFact adjacency BuildGoReachability uses, plus a required-layer node
// removal step layered on top. It is versioned independently of both
// CallGraphAlgorithm and ReachabilityAlgorithm since the removal/witness
// strategy can evolve without either of those changing.
const LayerBypassAlgorithm = "go-layer-bypass-registry@1"

// Stable diagnostic codes for LayerBypassResult.Coverage.Diagnostics[i].Code.
const (
	DiagLayerBypassBudgetExceeded   = "project_layer_bypass_budget_exceeded"
	DiagLayerBypassSourceLoadFailed = "project_layer_bypass_source_load_failed"
	// DiagLayerBypassAmbiguousLayer marks a RequiredLayer BuildGoLayerBypass
	// could not use with confidence: either it has no Prefixes at all, or
	// its Prefixes match no local package anywhere in the snapshot. Either
	// way, removing "the required layer" from the call graph would be a
	// no-op, so every witness for this run is suppressed rather than risk
	// reporting an ordinary reachable path as a bypass.
	DiagLayerBypassAmbiguousLayer = "project_layer_bypass_ambiguous_layer"
)

// LayerBypassConfidence classifies how a LayerBypassWitness's Path was
// derived.
type LayerBypassConfidence string

// LayerBypassConfidenceHigh is the only value BuildGoLayerBypass ever
// produces: a witness is only ever emitted when every hop on Path is a
// direct, statically resolved CallFact edge and the required-layer
// classification used to remove nodes was unambiguous (see
// DiagLayerBypassAmbiguousLayer). Anything less certain is suppressed
// entirely rather than emitted at a lower confidence.
const LayerBypassConfidenceHigh LayerBypassConfidence = "high"

// LayerBypassStep is one node in a LayerBypassWitness's Path, ordered from
// source to sink. Path and Line carry that node's repository-relative
// declaration position (the same position fnPosition already resolves to
// classify the node against RequiredLayer) so a consumer can anchor on real
// source data instead of the NodeID call-graph identity alone. Both are left
// zero-valued when the node has no resolvable local position -- expected for
// the sink, which is always a stdlib function (e.g.
// (*database/sql.DB).Query) with no declaration in the snapshot.
type LayerBypassStep struct {
	NodeID string `json:"node_id"`
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
}

// BypassLayer names a single required intermediate layer and its
// repository-relative directory prefixes, using the same directory-prefix
// semantics as codesignal.ArchitectureLayer (mirrored here, not imported --
// pkg/projectmodel never imports pkg/codesignal). Prefixes are matched
// exactly as layerContainsDir does: "." matches every directory, otherwise a
// prefix matches a directory that equals it or has it as a "/"-separated
// ancestor.
type BypassLayer struct {
	Name     string
	Prefixes []string
}

// LayerBypassWitness is one emitted layer-bypass observation: a source
// registry entry has a statically resolved call path to a sink registry
// entry that survives removing every node classified under RequiredLayer
// from the call graph -- i.e. a route from Source to Sink that structurally
// never passes through RequiredLayer. The absence of a witness for a given
// source/sink pair means only "no bypass path was found within the coverage
// this run achieved" (see LayerBypassResult.Coverage); it is never a "safe"
// or "compliant" claim, and it does not require a separate compliant path to
// coexist.
type LayerBypassWitness struct {
	ID               string                `json:"id"`
	Source           string                `json:"source"`
	Sink             string                `json:"sink"`
	RequiredLayer    string                `json:"required_layer"`
	Path             []LayerBypassStep     `json:"path"`
	Confidence       LayerBypassConfidence `json:"confidence"`
	AlgorithmVersion string                `json:"algorithm_version"`
}

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

// LayerBypassResult is the bounded, versioned set of LayerBypassWitnesses
// BuildGoLayerBypass produces for a Snapshot, plus the coverage measurement
// needed to judge how much of the source x sink space was actually
// searched.
type LayerBypassResult struct {
	// Witnesses holds one entry per source/sink pair with a statically
	// resolved bypass path found within budget. Sorted by Source then Sink.
	Witnesses []LayerBypassWitness
	// Sources holds every registry source function identified in the
	// snapshot, sorted -- identical to ReachabilityResult.Sources.
	Sources   []string
	Algorithm string
	Coverage  Coverage
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
func BuildGoLayerBypass(ctx context.Context, snapshot fs.FS, opts LayerBypassOptions) (LayerBypassResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Budgets.WallTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Budgets.WallTime)
		defer cancel()
	}

	start := time.Now()
	var memBefore, memAfter runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	loaded, err := loadGoSnapshot(ctx, snapshot, opts.Roots, opts.Budgets)
	if err != nil {
		return LayerBypassResult{}, fmt.Errorf("projectmodel: building call graph for layer bypass: %w", err)
	}
	defer loaded.cleanup()

	callGraph := buildGoCallGraphFromLoaded(ctx, loaded, CallGraphOptions{Roots: opts.Roots, Budgets: opts.Budgets})
	sources, sourcesComplete, rawSourceDiagnostics := findGoReachabilitySourcesFromLoaded(ctx, loaded)
	sourceDiagnostics := remapDiagnosticCodes(rawSourceDiagnostics, map[string]string{
		DiagReachabilityBudgetExceeded:   DiagLayerBypassBudgetExceeded,
		DiagReachabilitySourceLoadFailed: DiagLayerBypassSourceLoadFailed,
	})

	nodePositions, dirsComplete, dirDiagnostics := layerBypassNodePackageDirsFromLoaded(ctx, loaded)

	adjacency := buildCallGraphAdjacency(callGraph.CallFacts)
	sinks := append([]string(nil), ReachabilitySinkPatterns...)
	sort.Strings(sinks)

	callGraphIncomplete := !callGraph.Coverage.Complete

	requiredLayerNodes := map[string]bool{}
	for node, pos := range nodePositions {
		if layerBypassContainsDir(opts.RequiredLayer, pos.Dir) {
			requiredLayerNodes[node] = true
		}
	}
	// ambiguousLayer guards against the false-positive case where an
	// unconfigured/unmatched RequiredLayer would remove nothing from
	// adjacency, silently turning this into an ordinary reachability search
	// that could misreport a genuinely compliant path as a bypass witness.
	ambiguousLayer := len(opts.RequiredLayer.Prefixes) == 0 || len(requiredLayerNodes) == 0

	bypassAdjacency := adjacency
	if !ambiguousLayer {
		bypassAdjacency = removeLayerNodesFromAdjacency(adjacency, requiredLayerNodes)
	}

	var witnesses []LayerBypassWitness
	evaluated, truncatedPairs, nodesVisited := 0, 0, 0
	truncatedSearch := callGraphIncomplete || !dirsComplete
	unclassifiedNodeSeen := false

	if ambiguousLayer {
		// An ambiguous RequiredLayer means the search below never runs for
		// any pair, so every pair is truncated (not evaluated) and the run
		// as a whole is incomplete -- this must not look like a genuinely
		// searched, fully-covered "no bypass found" result.
		truncatedPairs = len(sources) * len(sinks)
		truncatedSearch = true
	} else {
		for _, source := range sources {
			if ctx.Err() != nil {
				truncatedSearch = true
				break
			}
			parents, hitBudget := bfsShortestPaths(ctx, source, bypassAdjacency, opts.MaxSearchNodes, &nodesVisited)
			if hitBudget {
				truncatedSearch = true
			}
			skip := hitBudget || ctx.Err() != nil || callGraphIncomplete || !dirsComplete
			for _, sink := range sinks {
				if skip {
					truncatedPairs++
					continue
				}
				evaluated++
				stepPath, ok := reconstructReachabilityPath(parents, source, sink)
				if !ok {
					continue
				}
				if !stepPathFullyClassified(stepPath, nodePositions) {
					// A node on the path has no resolvable package
					// directory (e.g. a synthetic wrapper), so its
					// required-layer membership is unknown -- suppress
					// rather than risk reporting a path that could have
					// been removed as an in-layer route.
					unclassifiedNodeSeen = true
					continue
				}
				witnesses = append(witnesses, LayerBypassWitness{
					ID:               fmt.Sprintf("bypass:%s:%s->%s@%s", opts.RequiredLayer.Name, source, sink, LayerBypassAlgorithm),
					Source:           source,
					Sink:             sink,
					RequiredLayer:    opts.RequiredLayer.Name,
					Path:             layerBypassSteps(stepPath, nodePositions),
					Confidence:       LayerBypassConfidenceHigh,
					AlgorithmVersion: LayerBypassAlgorithm,
				})
			}
		}
	}
	if !truncatedSearch && ctx.Err() != nil {
		truncatedSearch = true
	}

	runtime.ReadMemStats(&memAfter)
	memDelta := int64(memAfter.TotalAlloc) - int64(memBefore.TotalAlloc)
	if memDelta < 0 {
		memDelta = 0
	}

	unresolvedCallSites := callGraph.Coverage.Counts["unresolved_interface"] +
		callGraph.Coverage.Counts["unresolved_function_value"] +
		callGraph.Coverage.Counts["unresolved_reflection"] +
		callGraph.Coverage.Counts["unresolved_framework_registration"]

	diagnostics := append([]Diagnostic{}, callGraph.Coverage.Diagnostics...)
	diagnostics = append(diagnostics, sourceDiagnostics...)
	diagnostics = append(diagnostics, dirDiagnostics...)
	if ambiguousLayer {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	} else if unclassifiedNodeSeen && !containsDiagnosticCode(diagnostics, DiagLayerBypassAmbiguousLayer) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	}
	if truncatedSearch && !containsDiagnosticCode(diagnostics, DiagLayerBypassBudgetExceeded) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded})
	}

	complete := callGraph.Coverage.Complete && sourcesComplete && dirsComplete && !truncatedSearch

	sort.Slice(witnesses, func(i, j int) bool {
		if witnesses[i].Source != witnesses[j].Source {
			return witnesses[i].Source < witnesses[j].Source
		}
		return witnesses[i].Sink < witnesses[j].Sink
	})

	return LayerBypassResult{
		Witnesses: witnesses,
		Sources:   sources,
		Algorithm: LayerBypassAlgorithm,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "go_layer_bypass",
			Complete: complete,
			Counts: map[string]int{
				"sources_identified":               len(sources),
				"sinks_pinned":                     len(sinks),
				"source_sink_pairs_total":          len(sources) * len(sinks),
				"source_sink_pairs_evaluated":      evaluated,
				"source_sink_pairs_truncated":      truncatedPairs,
				"witnesses_found":                  len(witnesses),
				"required_layer_nodes_matched":     len(requiredLayerNodes),
				"underlying_call_sites_seen":       callGraph.Coverage.Counts["call_sites_seen"],
				"underlying_unresolved_call_sites": unresolvedCallSites,
				"ssa_programs_built":               loaded.programsBuilt(),
				"search_nodes_visited":             nodesVisited,
				"runtime_ms":                       int(time.Since(start) / time.Millisecond),
				"memory_bytes":                     int(memDelta), // coarse process-wide TotalAlloc delta; see BuildGoReachability's doc comment.
			},
			Budgets:     effectiveLayerBypassBudgets(opts),
			Diagnostics: diagnostics,
		}),
	}, nil
}

// layerBypassContainsDir mirrors codesignal's layerContainsDir: "." matches
// every directory, otherwise a prefix matches dir itself or any "/"-
// separated descendant of it.
func layerBypassContainsDir(layer BypassLayer, dir string) bool {
	for _, prefix := range layer.Prefixes {
		if prefix == "." || dir == prefix || (len(dir) > len(prefix) && dir[:len(prefix)+1] == prefix+"/") {
			return true
		}
	}
	return false
}

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
func layerBypassSteps(stepPath []ReachabilityStep, nodePositions map[string]layerBypassNodePosition) []LayerBypassStep {
	steps := make([]LayerBypassStep, len(stepPath))
	for i, step := range stepPath {
		ls := LayerBypassStep{NodeID: step.NodeID}
		if pos, ok := nodePositions[step.NodeID]; ok {
			ls.Path = pos.File
			ls.Line = pos.Line
		}
		steps[i] = ls
	}
	return steps
}

// removeLayerNodesFromAdjacency returns adjacency with every node in
// removed deleted, both as an edge source and as an edge destination, so a
// BFS over the result can only find a path that never touches one of those
// nodes. Surviving neighbor lists stay in the same sorted order adjacency
// already used, preserving bfsShortestPaths' deterministic tie-breaking.
func removeLayerNodesFromAdjacency(adjacency map[string][]string, removed map[string]bool) map[string][]string {
	out := make(map[string][]string, len(adjacency))
	for from, tos := range adjacency {
		if removed[from] {
			continue
		}
		var kept []string
		for _, to := range tos {
			if removed[to] {
				continue
			}
			kept = append(kept, to)
		}
		if len(kept) > 0 {
			out[from] = kept
		}
	}
	return out
}

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

	complete := loaded.discovery.Complete
	var diagnostics []Diagnostic
	positions := map[string]layerBypassNodePosition{}

	for _, root := range loaded.roots {
		if ctx.Err() != nil {
			complete = false
			diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded, Path: root.dir})
			break
		}
		if root.loadErr != nil {
			complete = false
			diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassSourceLoadFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
			continue
		}

		for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
			if ctx.Err() != nil {
				complete = false
				diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded, Path: root.dir})
				break
			}
			pos, ok := fnPosition(loaded.tempDir, fn)
			if !ok {
				continue
			}
			positions[fn.RelString(nil)] = pos
		}
	}

	return positions, complete, diagnostics
}

// fnPosition resolves fn's declaration position to a repository-relative
// package directory, file, and 1-based line, stripping materializeSnapshot's
// absolute tempDir prefix the same way relCallSitePath does for call sites.
// It reports false for a function with no resolvable position (e.g. a
// synthetic wrapper).
func fnPosition(tempDir string, fn *ssa.Function) (layerBypassNodePosition, bool) {
	pos := fn.Prog.Fset.Position(fn.Pos())
	if pos.Filename == "" {
		return layerBypassNodePosition{}, false
	}
	rel, err := filepath.Rel(tempDir, pos.Filename)
	if err != nil {
		return layerBypassNodePosition{}, false
	}
	relSlash := filepath.ToSlash(rel)
	return layerBypassNodePosition{Dir: path.Dir(relSlash), File: relSlash, Line: pos.Line}, true
}

// remapDiagnosticCodes returns a copy of diags with every Code present in
// codes rewritten to its mapped value, leaving any other diagnostic
// untouched. It is used to fold a shared helper's diagnostics (e.g.
// findGoReachabilitySources') into this evaluator's own diagnostic-code
// vocabulary rather than leaking a different feature's codes into
// LayerBypassResult.Coverage.
func remapDiagnosticCodes(diags []Diagnostic, codes map[string]string) []Diagnostic {
	if len(diags) == 0 {
		return diags
	}
	out := make([]Diagnostic, len(diags))
	for i, d := range diags {
		if mapped, ok := codes[d.Code]; ok {
			d.Code = mapped
		}
		out[i] = d
	}
	return out
}

// effectiveLayerBypassBudgets renders opts as LayerBypassResult.Coverage.Budgets,
// mirroring effectiveReachabilityBudgets exactly: the full
// EffectiveGoBudgets(opts.Budgets) vocabulary plus a "search_nodes" key for
// MaxSearchNodes.
func effectiveLayerBypassBudgets(opts LayerBypassOptions) map[string]int {
	budgets := EffectiveGoBudgets(opts.Budgets)
	budgets["search_nodes"] = opts.MaxSearchNodes
	return budgets
}
