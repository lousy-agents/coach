package codesignal

import (
	"sort"
)

// sortProjectChanges returns classified changes ordered by SemanticKey, ties
// broken by RuleID, mirroring sortSignals's deterministic-output guarantee.
// Nested related_locations, coverage_refs, and path-step source_locations are
// canonicalized into a fresh slice; path_steps themselves keep producer order.
func sortProjectChanges(changes []ProjectChange) []ProjectChange {
	out := make([]ProjectChange, len(changes))
	for i := range changes {
		out[i] = withCanonicalProjectChangeArrays(changes[i])
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.SemanticKey != b.SemanticKey {
			return a.SemanticKey < b.SemanticKey
		}
		return a.RuleID < b.RuleID
	})
	return out
}

// sortProjectFacts returns facts-only observations with a total order over
// every serialized field so equivalent analyses remain byte-identical even
// when producers omit or duplicate semantic keys (F-004).
func sortProjectFacts(facts []ProjectFact) []ProjectFact {
	out := make([]ProjectFact, len(facts))
	for i := range facts {
		out[i] = withCanonicalProjectFactArrays(facts[i])
	}
	sort.SliceStable(out, func(i, j int) bool {
		return compareProjectFacts(out[i], out[j]) < 0
	})
	return out
}

func withCanonicalProjectChangeArrays(in ProjectChange) ProjectChange {
	// Copy nested slices before rewriting so Build never mutates caller Input
	// headers shared after shallow ProjectChange copies from classify.
	return ProjectChange{
		SemanticKey:          in.SemanticKey,
		ID:                   in.ID,
		Fingerprint:          in.Fingerprint,
		RuleID:               in.RuleID,
		RuleVersion:          in.RuleVersion,
		BackendVersion:       in.BackendVersion,
		AlgorithmVersion:     in.AlgorithmVersion,
		ConfigDigest:         in.ConfigDigest,
		Kind:                 in.Kind,
		Category:             in.Category,
		Severity:             in.Severity,
		Confidence:           in.Confidence,
		Lifecycle:            in.Lifecycle,
		Changed:              in.Changed,
		CausalEvidenceDigest: in.CausalEvidenceDigest,
		PrimaryAnchor:        in.PrimaryAnchor,
		RelatedLocations:     canonicalProjectLocations(in.RelatedLocations),
		PathSteps:            canonicalPathSteps(in.PathSteps),
		CoverageRefs:         canonicalStringSlice(in.CoverageRefs),
		Evidence:             in.Evidence,
		MachineEvidence:      in.MachineEvidence,
		WhyItMatters:         in.WhyItMatters,
		Recommendation:       in.Recommendation,
		SuggestedSkill:       in.SuggestedSkill,
		Provenance:           in.Provenance,
	}
}

func withCanonicalProjectFactArrays(in ProjectFact) ProjectFact {
	return ProjectFact{
		Kind:         in.Kind,
		SemanticKey:  in.SemanticKey,
		PathSteps:    canonicalPathSteps(in.PathSteps),
		CoverageRefs: canonicalStringSlice(in.CoverageRefs),
		Evidence:     in.Evidence,
		Provenance:   in.Provenance,
	}
}

func canonicalPathSteps(in []ProjectPathStep) []ProjectPathStep {
	if len(in) == 0 {
		return in
	}
	out := append([]ProjectPathStep(nil), in...)
	for i := range out {
		out[i].SourceLocations = canonicalProjectLocations(out[i].SourceLocations)
	}
	return out
}

func canonicalProjectLocations(in []ProjectLocation) []ProjectLocation {
	if len(in) == 0 {
		return in
	}
	out := append([]ProjectLocation(nil), in...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return compareLocationValue(a.Location, b.Location) < 0
	})
	return out
}
