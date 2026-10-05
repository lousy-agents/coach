package codesignal

import (
	"strings"
)

// classifyProjectChanges computes identity, lifecycle, and causal Changed
// state for every project change on either side of a comparison. When
// lifecycleIndeterminate is true, no observation is promoted to introduced,
// existing, or resolved because one of the compared project models is not
// complete. Independently, a change that would be introduced at a
// rename/copy destination, or resolved at its source, is "unknown" because
// continuity identifies neither a fix nor a new defect. Duplicate
// SemanticKeys on either side produce diagnostics and keep the first
// occurrence only.
func classifyProjectChanges(hasBase bool, state projectLifecycleIndeterminacy, continuity undeterminedContinuity, headChanges, baseChanges []ProjectChange, noBaseLifecycle Lifecycle) ([]ProjectChange, []Diagnostic) {
	headByKey, headDiags := indexProjectChangesByKey(headChanges)
	baseByKey, baseDiags := indexProjectChangesByKey(baseChanges)
	diagnostics := append(headDiags, baseDiags...)

	lifecycleIndeterminate := state.any()
	// Several changes can share one anchor; their per-path diagnostics would
	// otherwise repeat byte for byte.
	seen := map[Diagnostic]struct{}{}

	result := make([]ProjectChange, 0, len(headByKey)+len(baseByKey))

	for _, key := range sortedProjectKeys(headByKey) {
		change := headByKey[key]
		baseChange, inBase := baseByKey[key]
		movedPath, moved := continuity.headPath(change)
		switch {
		case lifecycleIndeterminate:
			change.Lifecycle = "unknown"
			change.Changed = false
			change.Evidence += indeterminateLifecycleEvidenceNote
			diagnostics = appendDistinctDiagnostics(diagnostics, seen, degradedProjectChangeDiagnostics(change.PrimaryAnchor.Path, state)...)
		case !hasBase:
			change.Lifecycle = noBaseLifecycle
			change.Changed = false
		case inBase:
			change.Lifecycle = "existing"
			change.Changed = projectChangeChanged(change, baseChange)
		case moved:
			change.Lifecycle = "unknown"
			change.Changed = false
			change.Evidence += undeterminedContinuityEvidenceNote
			diagnostics = appendDistinctDiagnostics(diagnostics, seen, continuityDegradedDiagnostic(movedPath, "head", state.headRevision))
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
		movedPath, moved := continuity.basePath(change)
		switch {
		case lifecycleIndeterminate || !hasBase:
			change.Lifecycle = "unknown"
			change.Evidence += indeterminateLifecycleEvidenceNote
			diagnostics = appendDistinctDiagnostics(diagnostics, seen, degradedProjectChangeDiagnostics(change.PrimaryAnchor.Path, state)...)
		case moved:
			change.Lifecycle = "unknown"
			change.Evidence += undeterminedContinuityEvidenceNote
			diagnostics = appendDistinctDiagnostics(diagnostics, seen, continuityDegradedDiagnostic(movedPath, "base", state.baseRevision))
		default:
			change.Lifecycle = "resolved"
		}
		change.Changed = false
		change.Fingerprint = computeProjectFingerprint(change)
		change.ID = computeProjectChangeID(change)
		result = append(result, change)
	}

	return result, diagnostics
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
func projectChangeChanged(head, base ProjectChange) bool {
	if head.CausalEvidenceDigest != "" || base.CausalEvidenceDigest != "" {
		return head.CausalEvidenceDigest != base.CausalEvidenceDigest
	}
	return head.PrimaryAnchor != base.PrimaryAnchor
}
