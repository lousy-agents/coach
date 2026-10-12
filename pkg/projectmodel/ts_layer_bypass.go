package projectmodel

import (
	"context"
	"io/fs"
	"sort"
)

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
// internal/codesignalcli/ts_project_revision.go's tsBypassCoverageForFold
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

func tsLayerBypassSinks(facts []ReachabilityFact) []string {
	seen := map[string]bool{}
	for _, f := range facts {
		seen[f.Sink] = true
	}
	return mapKeysSorted(seen)
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
