package codesignal

import (
	"sort"
	"strings"
)

// indexProjectChangesByKey maps changes by SemanticKey, keeping the first
// occurrence. Duplicates violate the one-observation-per-key producer
// invariant and yield project_duplicate_semantic_key diagnostics rather than
// silent last-write-wins. Unlike Signal's groupAndOrder (which tolerates
// several signals sharing one composite key and assigns occurrence ordinals),
// ProjectChange's SemanticKey is itself the lifecycle identity.

// indeterminateLifecycleEvidenceNote is appended to a ProjectChange's own
// Evidence whenever classifyProjectChanges degrades it to lifecycle
// "unknown", so a reader scanning findings one at a time can see why without
// separately cross-referencing the project_lifecycle_indeterminate
// diagnostic in the report's top-level Diagnostics[]. Evidence is excluded
// from fingerprint/ID computation (see appendProjectIdentity), so this never
// affects lifecycle identity or Changed.
const indeterminateLifecycleEvidenceNote = ` (lifecycle is "unknown": see the project_lifecycle_indeterminate diagnostic for why)`

// classifyProjectChanges computes identity, lifecycle, and causal Changed
// state for every project change on either side of a comparison. When
// lifecycleIndeterminate is true, no observation is promoted to introduced,
// existing, or resolved because one of the compared project models is not
// complete. Duplicate SemanticKeys on either side produce diagnostics and
// keep the first occurrence only.

// !hasBase only reaches here when baseByKey is non-empty, which
// projectLifecycleState (codesignal.go) already treats as
// lifecycleIndeterminate on its own -- so this branch always
// runs with lifecycleIndeterminate true, and the note is never
// misattributed to a determinate change.

func sortedProjectKeys(byKey map[string]ProjectChange) []string {
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortProjectChanges returns classified changes ordered by SemanticKey, ties
// broken by RuleID, mirroring sortSignals's deterministic-output guarantee.
// Nested related_locations, coverage_refs, and path-step source_locations are
// canonicalized into a fresh slice; path_steps themselves keep producer order.

// sortProjectFacts returns facts-only observations with a total order over
// every serialized field so equivalent analyses remain byte-identical even
// when producers omit or duplicate semantic keys (F-004).

func comparePathSteps(a, b ProjectPathStep) int {
	if c := strings.Compare(a.NodeID, b.NodeID); c != 0 {
		return c
	}
	if c := strings.Compare(a.DisplayName, b.DisplayName); c != 0 {
		return c
	}
	if c := strings.Compare(a.Resolution, b.Resolution); c != 0 {
		return c
	}
	if c := strings.Compare(string(a.Confidence), string(b.Confidence)); c != 0 {
		return c
	}
	n := len(a.SourceLocations)
	if len(b.SourceLocations) < n {
		n = len(b.SourceLocations)
	}
	for i := 0; i < n; i++ {
		la, lb := a.SourceLocations[i], b.SourceLocations[i]
		if c := strings.Compare(la.Path, lb.Path); c != 0 {
			return c
		}
		if c := compareLocationValue(la.Location, lb.Location); c != 0 {
			return c
		}
	}
	switch {
	case len(a.SourceLocations) < len(b.SourceLocations):
		return -1
	case len(a.SourceLocations) > len(b.SourceLocations):
		return 1
	default:
		return 0
	}
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
