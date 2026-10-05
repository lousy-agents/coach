package codesignal

import (
	"sort"
)

// indexProjectChangesByKey maps changes by SemanticKey, keeping the first
// occurrence. Duplicates violate the one-observation-per-key producer
// invariant and yield project_duplicate_semantic_key diagnostics rather than
// silent last-write-wins. Unlike Signal's groupAndOrder (which tolerates
// several signals sharing one composite key and assigns occurrence ordinals),
// ProjectChange's SemanticKey is itself the lifecycle identity.
func indexProjectChangesByKey(changes []ProjectChange) (map[string]ProjectChange, []Diagnostic) {
	byKey := make(map[string]ProjectChange, len(changes))
	var diagnostics []Diagnostic
	for _, change := range changes {
		if _, exists := byKey[change.SemanticKey]; exists {
			diagnostics = append(diagnostics, Diagnostic{
				Path: change.PrimaryAnchor.Path,
				Kind: "project_duplicate_semantic_key",
				Message: "duplicate project observation semantic_key \"" + change.SemanticKey +
					"\"; keeping the first occurrence",
			})
			continue
		}
		byKey[change.SemanticKey] = change
	}
	return byKey, diagnostics
}

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
func canonicalStringSlice(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
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
