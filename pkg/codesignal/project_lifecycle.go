package codesignal

import (
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/semantics"
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

// classifyProjectChanges computes identity, lifecycle, and causal Changed
// state for every project change on either side of a comparison. When
// lifecycleIndeterminate is true, no observation is promoted to introduced,
// existing, or resolved because one of the compared project models is not
// complete. Duplicate SemanticKeys on either side produce diagnostics and
// keep the first occurrence only.
func classifyProjectChanges(hasBase, lifecycleIndeterminate bool, headChanges, baseChanges []ProjectChange, noBaseLifecycle Lifecycle) ([]ProjectChange, []Diagnostic) {
	headByKey, headDiags := indexProjectChangesByKey(headChanges)
	baseByKey, baseDiags := indexProjectChangesByKey(baseChanges)
	diagnostics := append(headDiags, baseDiags...)

	result := make([]ProjectChange, 0, len(headByKey)+len(baseByKey))

	for _, key := range sortedProjectKeys(headByKey) {
		change := headByKey[key]
		baseChange, inBase := baseByKey[key]
		switch {
		case lifecycleIndeterminate:
			change.Lifecycle = "unknown"
			change.Changed = false
		case !hasBase:
			change.Lifecycle = noBaseLifecycle
			change.Changed = false
		case inBase:
			change.Lifecycle = "existing"
			change.Changed = projectChangeChanged(change, baseChange)
		default:
			change.Lifecycle = "introduced"
			change.Changed = true
		}
		change.Fingerprint = computeProjectFingerprint(change)
		change.ID = computeProjectChangeID(change)
		result = append(result, change)
	}

	for _, key := range sortedProjectKeys(baseByKey) {
		if _, inHead := headByKey[key]; inHead {
			continue
		}
		change := baseByKey[key]
		if lifecycleIndeterminate || !hasBase {
			change.Lifecycle = "unknown"
		} else {
			change.Lifecycle = "resolved"
		}
		change.Changed = false
		change.Fingerprint = computeProjectFingerprint(change)
		change.ID = computeProjectChangeID(change)
		result = append(result, change)
	}

	return result, diagnostics
}

func projectChangeChanged(head, base ProjectChange) bool {
	if head.CausalEvidenceDigest != "" || base.CausalEvidenceDigest != "" {
		return head.CausalEvidenceDigest != base.CausalEvidenceDigest
	}
	return head.PrimaryAnchor != base.PrimaryAnchor
}

func sortedProjectKeys(byKey map[string]ProjectChange) []string {
	keys := make([]string, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// sortProjectChanges sorts classified changes by SemanticKey, ties broken
// by RuleID, mirroring sortSignals's deterministic-output guarantee. Nested
// related_locations, coverage_refs, and path-step source_locations are also
// canonicalized; path_steps themselves keep producer order (witness path).
func sortProjectChanges(changes []ProjectChange) {
	for i := range changes {
		canonicalizeProjectChangeArrays(&changes[i])
	}
	sort.SliceStable(changes, func(i, j int) bool {
		a, b := changes[i], changes[j]
		if a.SemanticKey != b.SemanticKey {
			return a.SemanticKey < b.SemanticKey
		}
		return a.RuleID < b.RuleID
	})
}

// sortProjectFacts sorts facts-only observations with a total order over every
// serialized field so equivalent analyses remain byte-identical even when
// producers omit or duplicate semantic keys (F-004).
func sortProjectFacts(facts []ProjectFact) {
	for i := range facts {
		canonicalizeProjectFactArrays(&facts[i])
	}
	sort.SliceStable(facts, func(i, j int) bool {
		return compareProjectFacts(facts[i], facts[j]) < 0
	})
}

func compareProjectFacts(a, b ProjectFact) int {
	if c := strings.Compare(a.Kind, b.Kind); c != 0 {
		return c
	}
	if c := strings.Compare(a.SemanticKey, b.SemanticKey); c != 0 {
		return c
	}
	if c := strings.Compare(a.Evidence, b.Evidence); c != 0 {
		return c
	}
	if c := strings.Compare(a.Provenance.Producer, b.Provenance.Producer); c != 0 {
		return c
	}
	if c := strings.Compare(a.Provenance.FindingKind, b.Provenance.FindingKind); c != 0 {
		return c
	}
	if c := compareStringSlices(a.CoverageRefs, b.CoverageRefs); c != 0 {
		return c
	}
	return comparePathStepSlices(a.PathSteps, b.PathSteps)
}

func compareStringSlices(a, b []string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := strings.Compare(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

func comparePathStepSlices(a, b []ProjectPathStep) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if c := comparePathSteps(a[i], b[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(a) < len(b):
		return -1
	case len(a) > len(b):
		return 1
	default:
		return 0
	}
}

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

func canonicalizeProjectChangeArrays(change *ProjectChange) {
	change.RelatedLocations = canonicalProjectLocations(change.RelatedLocations)
	change.CoverageRefs = canonicalStringSlice(change.CoverageRefs)
	// PathSteps is a shared slice header after a shallow ProjectChange copy
	// (classify copies structs, not nested slices). Copy before rewriting
	// SourceLocations so Build never mutates the caller's Input.
	change.PathSteps = canonicalPathSteps(change.PathSteps)
}

func canonicalizeProjectFactArrays(fact *ProjectFact) {
	fact.CoverageRefs = canonicalStringSlice(fact.CoverageRefs)
	fact.PathSteps = canonicalPathSteps(fact.PathSteps)
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

func canonicalStringSlice(in []string) []string {
	if len(in) == 0 {
		return in
	}
	out := append([]string(nil), in...)
	sort.Strings(out)
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

func compareLocationValue(a, b semantics.Location) int {
	switch {
	case a.StartRow != b.StartRow:
		return compareUint(a.StartRow, b.StartRow)
	case a.StartCol != b.StartCol:
		return compareUint(a.StartCol, b.StartCol)
	case a.EndRow != b.EndRow:
		return compareUint(a.EndRow, b.EndRow)
	case a.EndCol != b.EndCol:
		return compareUint(a.EndCol, b.EndCol)
	case a.StartByte != b.StartByte:
		return compareUint(a.StartByte, b.StartByte)
	case a.EndByte != b.EndByte:
		return compareUint(a.EndByte, b.EndByte)
	default:
		return 0
	}
}
