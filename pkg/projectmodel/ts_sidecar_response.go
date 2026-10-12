package projectmodel

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

func modelFromTSSidecarResponse(meta SnapshotMeta, opts TSSidecarOptions, resp projectbridge.Response, files []projectbridge.ProjectFile) Model {
	rootScopes := rootScopesFromWire(resp.RootScopes)
	complete := resp.Coverage.Complete
	diagnostics := diagnosticsFromWire(resp.Coverage.Diagnostics)
	if gaps := rootScopeIncompleteDiagnostics(rootScopes); len(gaps) > 0 {
		complete = false
		diagnostics = append(diagnostics, gaps...)
	}

	return Model{
		SchemaVersion:     SchemaVersion,
		Repository:        meta.Repository,
		Snapshot:          tsSidecarSnapshot(meta, opts),
		Workspaces:        tsWorkspaceFactsFromCollected(files),
		Files:             tsFileFactsFromCollected(files),
		ImportEdges:       importEdgesFromWire(resp.ImportEdges),
		CallFacts:         callFactsFromWire(resp.CallGraph),
		ReachabilityFacts: reachabilityFactsFromWire(resp.ReachabilityFacts),
		RootScopes:        rootScopes,
		Coverage: canonicalCoverage(Coverage{
			Phase:       tsSidecarPhase,
			Complete:    complete,
			Counts:      resp.Coverage.Counts,
			Budgets:     resp.Coverage.Budgets,
			Diagnostics: diagnostics,
		}),
	}
}

func tsSidecarErrorDiagnostics(resp projectbridge.Response) []Diagnostic {
	message := resp.Error.Message
	if resp.Error.Kind != "" {
		message = fmt.Sprintf("%s (kind: %s)", message, resp.Error.Kind)
	}
	diags := []Diagnostic{{Code: DiagBackendUnavailable, Message: message}}
	return append(diags, diagnosticsFromWire(resp.Coverage.Diagnostics)...)
}

func importEdgesFromWire(in []projectbridge.ImportEdgeFact) []ImportEdge {
	edges := make([]ImportEdge, 0, len(in))
	for _, e := range in {
		edges = append(edges, ImportEdge{From: e.From, To: e.To, Kind: e.Kind, Site: e.Site, Resolution: e.Resolution})
	}
	return edges
}

func callFactsFromWire(in []projectbridge.CallGraphEdgeFact) []CallFact {
	facts := make([]CallFact, 0, len(in))
	for _, f := range in {
		facts = append(facts, CallFact{From: f.From, To: f.To})
	}
	return facts
}

func reachabilityFactsFromWire(in []projectbridge.ReachabilityFactWire) []ReachabilityFact {
	facts := make([]ReachabilityFact, 0, len(in))
	for _, f := range in {
		facts = append(facts, ReachabilityFact{
			ID:               f.ID,
			Kind:             f.Kind,
			Confidence:       ReachabilityConfidence(f.Confidence),
			Source:           f.Source,
			Sink:             f.Sink,
			Path:             reachabilityStepsFromWire(f.Path),
			AlgorithmVersion: f.AlgorithmVersion,
		})
	}
	return facts
}

func reachabilityStepsFromWire(in []projectbridge.ReachabilityStepFact) []ReachabilityStep {
	steps := make([]ReachabilityStep, 0, len(in))
	for _, s := range in {
		steps = append(steps, ReachabilityStep{NodeID: s.NodeID})
	}
	return steps
}

func diagnosticsFromWire(in []projectbridge.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(in))
	for _, d := range in {
		out = append(out, Diagnostic{Code: d.Code, Message: d.Message, Path: d.Path})
	}
	return out
}
