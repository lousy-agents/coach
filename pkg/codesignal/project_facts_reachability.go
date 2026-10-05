package codesignal

import "github.com/lousy-agents/coach/pkg/domain"

// ReachabilityProjectFacts maps result's possible-call-reachability facts
// (see pkg/projectmodel.BuildGoReachability / BuildTypeScriptReachability)
// onto ProjectFact. Provenance.Producer is always "projectmodel",
// Provenance.FindingKind is the fact's own Kind, and Provenance.Language is
// set from the language parameter ("go" or "typescript"), matching this
// package's other project-origin emitters (rule_layer_violation.go,
// rule_layer_bypass.go). The output is facts-only by construction -- it
// builds no ProjectChange or Signal, since ordinary possible-call
// reachability is never an active finding (issue #216 AC-4/AC-11).
func ReachabilityProjectFacts(result domain.ReachabilityResult, language string) []ProjectFact {
	facts := make([]ProjectFact, 0, len(result.Facts))
	for _, fact := range result.Facts {
		facts = append(facts, ProjectFact{
			Kind:        fact.Kind,
			SemanticKey: reachabilityFactSemanticKey(fact),
			PathSteps:   reachabilityFactPathSteps(fact),
			Evidence:    reachabilityFactEvidence(fact),
			Provenance:  Provenance{Producer: "projectmodel", FindingKind: fact.Kind, Language: language},
		})
	}
	return facts
}

// reachabilityFactSemanticKey mirrors layerBypassChange's
// "<kind>:<source>-><sink>" SemanticKey convention (rule_layer_bypass.go),
// keyed on Kind/Source/Sink so identity stays stable across a Path change
// between two revisions.
func reachabilityFactSemanticKey(fact domain.ReachabilityFact) string {
	return fact.Kind + ":" + fact.Source + "->" + fact.Sink
}

func reachabilityFactPathSteps(fact domain.ReachabilityFact) []ProjectPathStep {
	if len(fact.Path) == 0 {
		return nil
	}
	confidence := reachabilityStepConfidence(fact.Confidence)
	steps := make([]ProjectPathStep, len(fact.Path))
	for i, step := range fact.Path {
		steps[i] = ProjectPathStep{NodeID: step.NodeID, Confidence: confidence}
	}
	return steps
}

// reachabilityStepConfidence renders a domain.ReachabilityConfidence
// in codesignal's high/medium/low vocabulary. ResolvedDirect is currently
// the only value either backend's reachability traversal produces (every
// hop is a statically resolved edge), so it maps to "high"; an unrecognized
// future value passes through unchanged rather than being silently dropped.
func reachabilityStepConfidence(c domain.ReachabilityConfidence) Confidence {
	if c == domain.ReachabilityConfidenceResolvedDirect {
		return Confidence("high")
	}
	return Confidence(c)
}

// reachabilityFactEvidence renders fact using the same "possible call"
// wording on both the Go and TypeScript backends (AC-1/AC-6/AC-11), never
// claiming a verified/sound dataflow result.
func reachabilityFactEvidence(fact domain.ReachabilityFact) string {
	return fact.Source + " has a possible call path to " + fact.Sink
}
