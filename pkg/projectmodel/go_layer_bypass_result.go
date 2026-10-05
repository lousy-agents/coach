package projectmodel

import (
	"sort"
	"time"
)

func assembleLayerBypassResult(loaded *loadedGoSnapshot, callGraph CallGraphResult, sources, sinks []string, requiredLayerNodes map[string]bool, search layerBypassSearch, sourceDiagnostics, dirDiagnostics []Diagnostic, sourcesComplete, dirsComplete, ambiguousLayer bool, opts LayerBypassOptions, start time.Time, memDelta int64) LayerBypassResult {
	diagnostics := append([]Diagnostic{}, callGraph.Coverage.Diagnostics...)
	diagnostics = append(diagnostics, sourceDiagnostics...)
	diagnostics = append(diagnostics, dirDiagnostics...)
	if ambiguousLayer {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	} else if search.unclassifiedNodeSeen && !containsDiagnosticCode(diagnostics, DiagLayerBypassAmbiguousLayer) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassAmbiguousLayer})
	}
	if search.truncatedSearch && !containsDiagnosticCode(diagnostics, DiagLayerBypassBudgetExceeded) {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagLayerBypassBudgetExceeded})
	}

	sort.Slice(search.witnesses, func(i, j int) bool {
		if search.witnesses[i].Source != search.witnesses[j].Source {
			return search.witnesses[i].Source < search.witnesses[j].Source
		}
		return search.witnesses[i].Sink < search.witnesses[j].Sink
	})

	return LayerBypassResult{
		Witnesses: search.witnesses,
		Sources:   sources,
		Algorithm: LayerBypassAlgorithm,
		Coverage: canonicalCoverage(Coverage{
			Phase:    "go_layer_bypass",
			Complete: callGraph.Coverage.Complete && sourcesComplete && dirsComplete && !search.truncatedSearch,
			Counts: map[string]int{
				"sources_identified":               len(sources),
				"sinks_pinned":                     len(sinks),
				"source_sink_pairs_total":          len(sources) * len(sinks),
				"source_sink_pairs_evaluated":      search.evaluated,
				"source_sink_pairs_truncated":      search.truncatedPairs,
				"witnesses_found":                  len(search.witnesses),
				"required_layer_nodes_matched":     len(requiredLayerNodes),
				"underlying_call_sites_seen":       callGraph.Coverage.Counts["call_sites_seen"],
				"underlying_unresolved_call_sites": unresolvedCallSiteCount(callGraph.Coverage.Counts),
				"ssa_programs_built":               loaded.programsBuilt(),
				"search_nodes_visited":             search.nodesVisited,
				"runtime_ms":                       int(time.Since(start) / time.Millisecond),
				"memory_bytes":                     int(memDelta),
			},
			Budgets:     effectiveLayerBypassBudgets(opts),
			Diagnostics: diagnostics,
		}),
	}
}
