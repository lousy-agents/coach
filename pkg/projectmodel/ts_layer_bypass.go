package projectmodel

import (
	"context"
	"fmt"
)

// Deprecated: call BuildTypeScriptModelViaSidecar once and pass the Model to
// BuildTypeScriptLayerBypassFromModel / BuildTypeScriptReachabilityFromModel
// instead, so multiple derivations share one sidecar round trip.

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

// tsLayerBypassRequiredLayerNodes decides the ambiguous-layer match from
// snapshotFiles, not from the returned node-position map: the real
// sidecar's call graph only ever contains route-handler-to-sink edges, so a
// required layer with no route handler of its own -- the exact shape a
// genuine layer bypass produces -- would never appear as a CallFact
// endpoint even though real files live there.

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
		result.searchSources(ctx, sources, sinks, adjacency, nodePositions, requiredLayer)
	}
	if !result.truncatedSearch && ctx.Err() != nil {
		result.truncatedSearch = true
	}
	return result
}

func (result *tsLayerBypassSearchResult) searchSources(ctx context.Context, sources, sinks []string, adjacency map[string][]string, nodePositions map[string]layerBypassNodePosition, requiredLayer BypassLayer) {
	budget := &bfsBudget{}
	for _, source := range sources {
		if ctx.Err() != nil {
			result.truncatedSearch = true
			break
		}
		sourceResult := tsLayerBypassSearchFromSource(ctx, source, sinks, adjacency, nodePositions, requiredLayer, budget)
		result.truncatedSearch = result.truncatedSearch || sourceResult.truncatedSearch
		result.truncatedPairs += sourceResult.truncatedPairs
		result.evaluated += sourceResult.evaluated
		result.unclassifiedNodeSeen = result.unclassifiedNodeSeen || sourceResult.unclassifiedNodeSeen
		result.witnesses = append(result.witnesses, sourceResult.witnesses...)
	}
	result.nodesVisited = budget.visited
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
// evaluates every sink against it. budget is the shared node-visit counter
// across every source in the search.
func tsLayerBypassSearchFromSource(
	ctx context.Context,
	source string,
	sinks []string,
	adjacency map[string][]string,
	nodePositions map[string]layerBypassNodePosition,
	requiredLayer BypassLayer,
	budget *bfsBudget,
) tsLayerBypassSourceResult {
	var result tsLayerBypassSourceResult
	parents, hitBudget := budget.shortestPaths(ctx, source, adjacency)
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

func tsLayerBypassSinks(facts []ReachabilityFact) []string {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Sink] = true
	}
	return mapKeysSorted(seen)
}

// tsLayerBypassNodePosition parses a TS call-graph node ID of the
// "file:<repo-relative path>#<name>" shape (reachability.ts's
// functionSourceId). It reports false for a synthetic sink node ID (e.g.
// "(PrismaClient).findMany"), which carries no file path at all.
