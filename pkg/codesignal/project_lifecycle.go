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

// DiagKindProjectChangeLifecycleIndeterminate identifies a diagnostic naming
// one degraded ProjectChange's own anchor path together with the comparison
// Side/Revision that made it indeterminate. It is additive to the single
// project_lifecycle_indeterminate diagnostic emitted once per report.
const DiagKindProjectChangeLifecycleIndeterminate = "project_change_lifecycle_indeterminate"

// projectLifecycleIndeterminacy breaks the indeterminacy condition out per
// cause so each degraded change can be attributed to the responsible side(s).
// baseIncomplete and inconsistentBase are mutually exclusive.
type projectLifecycleIndeterminacy struct {
	headIncomplete   bool
	baseIncomplete   bool
	inconsistentBase bool
	headRevision     string
	baseRevision     string
}

func (s projectLifecycleIndeterminacy) any() bool {
	return s.headIncomplete || s.baseIncomplete || s.inconsistentBase
}

// degradedProjectChangeDiagnostics emits one Diagnostic per side the state
// implicates, so a consumer keying off Side never misses a side when both
// are independently incomplete.
func degradedProjectChangeDiagnostics(path string, state projectLifecycleIndeterminacy) []Diagnostic {
	var diagnostics []Diagnostic
	if state.headIncomplete {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": " + sideRevisionLabel("head", state.headRevision) + " project analysis coverage is incomplete",
			Side:     "head",
			Revision: state.headRevision,
		})
	}
	if state.baseIncomplete {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": " + sideRevisionLabel("base", state.baseRevision) + " project analysis coverage is incomplete",
			Side:     "base",
			Revision: state.baseRevision,
		})
	}
	if state.inconsistentBase {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": " + sideRevisionLabel("base", state.baseRevision) + " supplied project observations without a completed base analysis",
			Side:     "base",
			Revision: state.baseRevision,
		})
	}
	return diagnostics
}

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
