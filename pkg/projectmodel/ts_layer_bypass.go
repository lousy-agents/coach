package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

const TSLayerBypassAlgorithm = "ts-layer-bypass-registry@1"

// Deprecated: call BuildTypeScriptModelViaSidecar once and pass the Model to
// BuildTypeScriptLayerBypassFromModel / BuildTypeScriptReachabilityFromModel
// instead, so multiple derivations share one sidecar round trip.
func BuildTypeScriptLayerBypass(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions, requiredLayer BypassLayer) (LayerBypassResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	model, err := BuildTypeScriptModelViaSidecar(ctx, snapshot, meta, opts)
	if err != nil {
		return LayerBypassResult{}, err
	}

	return BuildTypeScriptLayerBypassFromModel(ctx, model, requiredLayer), nil
}

// BuildTypeScriptLayerBypassFromModel's per-pair witness evaluation is never
// gated on model.Coverage.Complete or any reachability-gap diagnostic: an
// unrelated source's own routine gap (e.g. a handler delegating one hop into
// a helper/service function) must never suppress a different,
// already-fully-resolved witness. LayerBypassResult.Coverage.Complete is
// instead computed independently below, so incompleteness is still reported
// honestly at the result level even though it never suppresses a witness.
//
// That result-level Coverage.Complete still conflates two distinct causes
// under one diagnostic code (DiagLayerBypassAmbiguousLayer covers both
// tsLayerBypassRequiredLayerNodes' genuine ambiguous-layer search failure
// and tsLayerBypassSearchFromSource's single-witness unclassified-node
// case; see tsLayerBypassDiagnostics). A caller that treats this
// LayerBypassResult as the customer-facing verdict must re-derive
// completeness per findable witness the way
// internal/codesignalcli/project_ts_backend.go's tsBypassCoverageForFold
// does, rather than trusting this field directly.
func BuildTypeScriptLayerBypassFromModel(budgetCtx context.Context, model Model, requiredLayer BypassLayer) LayerBypassResult {
	if budgetCtx == nil {
		budgetCtx = context.Background()
	}

	sources := tsReachabilitySourcesReachedASink(model.ReachabilityFacts)
	sinks := tsLayerBypassSinks(model.ReachabilityFacts)
	adjacency := buildCallGraphAdjacency(model.CallFacts)
	nodePositions := tsLayerBypassNodePositions(model.CallFacts)

	requiredLayerNodes, ambiguousLayer := tsLayerBypassRequiredLayerNodes(model.Files, nodePositions, requiredLayer)

	bypassAdjacency := adjacency
	if !ambiguousLayer {
		bypassAdjacency = removeLayerNodesFromAdjacency(adjacency, requiredLayerNodes)
	}

	search := tsLayerBypassRunSearch(budgetCtx, sources, sinks, bypassAdjacency, nodePositions, requiredLayer, ambiguousLayer)

	diagnostics := tsLayerBypassDiagnostics(model.Coverage.Diagnostics, ambiguousLayer, search.unclassifiedNodeSeen, search.truncatedSearch)

	complete := model.Coverage.Complete && !tsReachabilityHasGap(model.Coverage.Diagnostics) && !search.truncatedSearch

	sort.Slice(search.witnesses, func(i, j int) bool {
		if search.witnesses[i].Source != search.witnesses[j].Source {
			return search.witnesses[i].Source < search.witnesses[j].Source
		}
		return search.witnesses[i].Sink < search.witnesses[j].Sink
	})

	return LayerBypassResult{
		Witnesses: search.witnesses,
		Sources:   sources,
		Algorithm: TSLayerBypassAlgorithm,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "ts_layer_bypass",
			Complete: complete,
			Counts: map[string]int{
				"sources_identified":           len(sources),
				"sinks_identified":             len(sinks),
				"source_sink_pairs_total":      len(sources) * len(sinks),
				"source_sink_pairs_evaluated":  search.evaluated,
				"source_sink_pairs_truncated":  search.truncatedPairs,
				"witnesses_found":              len(search.witnesses),
				"required_layer_nodes_matched": len(requiredLayerNodes),
				"search_nodes_visited":         search.nodesVisited,
			},
			Diagnostics: diagnostics,
		}),
	}
}

// tsLayerBypassRequiredLayerNodes decides the ambiguous-layer match from
// snapshotFiles, not from the returned node-position map: the real
// sidecar's call graph only ever contains route-handler-to-sink edges, so a
// required layer with no route handler of its own -- the exact shape a
// genuine layer bypass produces -- would never appear as a CallFact
// endpoint even though real files live there.
func tsLayerBypassRequiredLayerNodes(snapshotFiles []File, callGraphNodePositions map[string]layerBypassNodePosition, requiredLayer BypassLayer) (map[string]bool, bool) {
	requiredLayerNodes := map[string]bool{}
	for node, pos := range callGraphNodePositions {
		if layerBypassContainsDir(requiredLayer, pos.Dir) {
			requiredLayerNodes[node] = true
		}
	}
	ambiguousLayer := len(requiredLayer.Prefixes) == 0 || !tsLayerBypassLayerMatchesFiles(snapshotFiles, requiredLayer)
	return requiredLayerNodes, ambiguousLayer
}

type tsLayerBypassSearchResult struct {
	witnesses            []LayerBypassWitness
	evaluated            int
	truncatedPairs       int
	nodesVisited         int
	truncatedSearch      bool
	unclassifiedNodeSeen bool
}

func tsLayerBypassRunSearch(ctx context.Context, sources, sinks []string, adjacency map[string][]string, nodePositions map[string]layerBypassNodePosition, requiredLayer BypassLayer, ambiguousLayer bool) tsLayerBypassSearchResult {
	var result tsLayerBypassSearchResult

	if ambiguousLayer {
		result.truncatedPairs = len(sources) * len(sinks)
		result.truncatedSearch = true
	} else {
		for _, source := range sources {
			if ctx.Err() != nil {
				result.truncatedSearch = true
				break
			}
			sourceResult := tsLayerBypassSearchFromSource(ctx, source, sinks, adjacency, nodePositions, requiredLayer, &result.nodesVisited)
			result.truncatedSearch = result.truncatedSearch || sourceResult.truncatedSearch
			result.truncatedPairs += sourceResult.truncatedPairs
			result.evaluated += sourceResult.evaluated
			result.unclassifiedNodeSeen = result.unclassifiedNodeSeen || sourceResult.unclassifiedNodeSeen
			result.witnesses = append(result.witnesses, sourceResult.witnesses...)
		}
	}
	if !result.truncatedSearch && ctx.Err() != nil {
		result.truncatedSearch = true
	}
	return result
}

// tsLayerBypassSourceResult is one source's contribution to a
// tsLayerBypassSearchResult; tsLayerBypassRunSearch accumulates it across
// every source.
type tsLayerBypassSourceResult struct {
	witnesses            []LayerBypassWitness
	evaluated            int
	truncatedPairs       int
	truncatedSearch      bool
	unclassifiedNodeSeen bool
}

// tsLayerBypassSearchFromSource runs source's BFS shortest-path tree and
// evaluates every sink against it. nodesVisited is bfsShortestPaths' own
// running budget counter, shared across every source in the search.
func tsLayerBypassSearchFromSource(
	ctx context.Context,
	source string,
	sinks []string,
	adjacency map[string][]string,
	nodePositions map[string]layerBypassNodePosition,
	requiredLayer BypassLayer,
	nodesVisited *int,
) tsLayerBypassSourceResult {
	var result tsLayerBypassSourceResult
	parents, hitBudget := bfsShortestPaths(ctx, source, adjacency, 0, nodesVisited)
	if hitBudget {
		result.truncatedSearch = true
	}
	skip := hitBudget || ctx.Err() != nil
	for _, sink := range sinks {
		if skip {
			result.truncatedPairs++
			continue
		}
		result.evaluated++
		stepPath, ok := reconstructReachabilityPath(parents, source, sink)
		if !ok {
			continue
		}
		if !stepPathFullyClassified(stepPath, nodePositions) {
			result.unclassifiedNodeSeen = true
			continue
		}
		result.witnesses = append(result.witnesses, LayerBypassWitness{
			ID:               fmt.Sprintf("bypass:%s:%s->%s@%s", requiredLayer.Name, source, sink, TSLayerBypassAlgorithm),
			Source:           source,
			Sink:             sink,
			RequiredLayer:    requiredLayer.Name,
			Path:             layerBypassSteps(stepPath, nodePositions),
			Confidence:       LayerBypassConfidenceHigh,
			AlgorithmVersion: TSLayerBypassAlgorithm,
		})
	}
	return result
}

func tsLayerBypassDiagnostics(base []Diagnostic, ambiguousLayer, unclassifiedNodeSeen, truncatedSearch bool) []Diagnostic {
	diagnostics := append([]Diagnostic{}, base...)
	if ambiguousLayer {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	} else if unclassifiedNodeSeen && !containsDiagnosticCode(diagnostics, DiagLayerBypassAmbiguousLayer) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	}
	if truncatedSearch && !containsDiagnosticCode(diagnostics, DiagLayerBypassBudgetExceeded) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded})
	}
	return diagnostics
}

func tsLayerBypassLayerMatchesFiles(files []File, layer BypassLayer) bool {
	for _, f := range files {
		if layerBypassContainsDir(layer, path.Dir(f.Path)) {
			return true
		}
	}
	return false
}

func tsLayerBypassSinks(facts []ReachabilityFact) []string {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Sink] = true
	}
	return mapKeysSorted(seen)
}

func tsLayerBypassNodePositions(facts []CallFact) map[string]layerBypassNodePosition {
	positions := map[string]layerBypassNodePosition{}
	for _, f := range facts {
		for _, nodeID := range [2]string{f.From, f.To} {
			if _, ok := positions[nodeID]; ok {
				continue
			}
			if pos, ok := tsLayerBypassNodePosition(nodeID); ok {
				positions[nodeID] = pos
			}
		}
	}
	return positions
}

// tsLayerBypassNodePosition parses a TS call-graph node ID of the
// "file:<repo-relative path>#<name>" shape (reachability.ts's
// functionSourceId). It reports false for a synthetic sink node ID (e.g.
// "(PrismaClient).findMany"), which carries no file path at all.
func tsLayerBypassNodePosition(nodeID string) (layerBypassNodePosition, bool) {
	rest, ok := strings.CutPrefix(nodeID, "file:")
	if !ok {
		return layerBypassNodePosition{}, false
	}
	filePath, _, ok := strings.Cut(rest, "#")
	if !ok || filePath == "" {
		return layerBypassNodePosition{}, false
	}
	return layerBypassNodePosition{Dir: path.Dir(filePath), File: filePath}, true
}
