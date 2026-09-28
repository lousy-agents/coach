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

// indeterminateLifecycleEvidenceNote is appended to a ProjectChange's own
// Evidence whenever classifyProjectChanges degrades it to lifecycle
// "unknown", so a reader scanning findings one at a time can see why without
// separately cross-referencing the project_lifecycle_indeterminate
// diagnostic in the report's top-level Diagnostics[]. Evidence is excluded
// from fingerprint/ID computation (see appendProjectIdentity), so this never
// affects lifecycle identity or Changed.
const indeterminateLifecycleEvidenceNote = ` (lifecycle is "unknown": see the project_lifecycle_indeterminate diagnostic for why)`

// DiagKindProjectChangeLifecycleIndeterminate identifies a diagnostic that
// names one specific degraded ProjectChange's own repository-relative path
// (change.PrimaryAnchor.Path) together with the comparison Side/Revision
// that made it indeterminate. This is additive to, not a replacement for,
// the single generic project_lifecycle_indeterminate diagnostic
// projectLifecycleState emits once per report and
// indeterminateLifecycleEvidenceNote's per-change Evidence pointer to it.
const DiagKindProjectChangeLifecycleIndeterminate = "project_change_lifecycle_indeterminate"

// projectLifecycleIndeterminacy carries the lifecycle-indeterminacy
// condition broken out per contributing cause, so classifyProjectChanges can
// attribute each degraded change's diagnostic to the specific comparison
// side(s) responsible instead of only knowing that something, somewhere,
// was wrong. baseIncomplete and inconsistentBase are mutually exclusive:
// baseIncomplete is only ever computed when ProjectBaseAnalyzed is true.
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

// degradedProjectChangeDiagnostics names path (a change already forced to
// lifecycle "unknown") with one Diagnostic per comparison side state
// implicates, so a consumer keying off Side never misses that a side was
// involved even when both head and base are independently incomplete.
func degradedProjectChangeDiagnostics(path string, state projectLifecycleIndeterminacy) []Diagnostic {
	var diagnostics []Diagnostic
	if state.headIncomplete {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": head revision " + state.headRevision + " project analysis coverage is incomplete",
			Side:     "head",
			Revision: state.headRevision,
		})
	}
	if state.baseIncomplete {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": base revision " + state.baseRevision + " project analysis coverage is incomplete",
			Side:     "base",
			Revision: state.baseRevision,
		})
	}
	if state.inconsistentBase {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": base revision " + state.baseRevision + " supplied project observations without a completed base analysis",
			Side:     "base",
			Revision: state.baseRevision,
		})
	}
	return diagnostics
}

// classifyProjectChanges computes identity, lifecycle, and causal Changed
// state for every project change on either side of a comparison. When
// state.any() is true, no observation is promoted to introduced, existing,
// or resolved because one of the compared project models is not complete;
// each degraded change additionally gets its own
// DiagKindProjectChangeLifecycleIndeterminate diagnostic(s) via
// degradedProjectChangeDiagnostics. Duplicate SemanticKeys on either side
// produce diagnostics and keep the first occurrence only.
func classifyProjectChanges(hasBase bool, state projectLifecycleIndeterminacy, headChanges, baseChanges []ProjectChange, noBaseLifecycle Lifecycle) ([]ProjectChange, []Diagnostic) {
	headByKey, headDiags := indexProjectChangesByKey(headChanges)
	baseByKey, baseDiags := indexProjectChangesByKey(baseChanges)
	diagnostics := append(headDiags, baseDiags...)

	lifecycleIndeterminate := state.any()

	result := make([]ProjectChange, 0, len(headByKey)+len(baseByKey))

	for _, key := range sortedProjectKeys(headByKey) {
		change := headByKey[key]
		baseChange, inBase := baseByKey[key]
		switch {
		case lifecycleIndeterminate:
			change.Lifecycle = "unknown"
			change.Changed = false
			change.Evidence += indeterminateLifecycleEvidenceNote
			diagnostics = append(diagnostics, degradedProjectChangeDiagnostics(change.PrimaryAnchor.Path, state)...)
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
			// !hasBase only reaches here when baseByKey is non-empty, which
			// projectLifecycleState (codesignal.go) already treats as
			// lifecycleIndeterminate on its own -- so this branch always
			// runs with lifecycleIndeterminate true, and the note is never
			// misattributed to a determinate change.
			change.Lifecycle = "unknown"
			change.Evidence += indeterminateLifecycleEvidenceNote
			diagnostics = append(diagnostics, degradedProjectChangeDiagnostics(change.PrimaryAnchor.Path, state)...)
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
