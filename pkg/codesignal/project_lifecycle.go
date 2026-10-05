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
			change.Evidence += indeterminateLifecycleEvidenceNote
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
			// projectLifecycleState (project_lifecycle_state.go) already treats as
			// lifecycleIndeterminate on its own -- so this branch always
			// runs with lifecycleIndeterminate true, and the note is never
			// misattributed to a determinate change.
			change.Lifecycle = "unknown"
			change.Evidence += indeterminateLifecycleEvidenceNote
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
