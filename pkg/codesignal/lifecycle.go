package codesignal

import "sort"

// signalKey groups signals for occurrence-ordinal assignment and lifecycle
// matching: (RuleID, normalized path, Subject, normalized evidence).
type signalKey struct {
	ruleID, path, subject, evidence string
}

// keyOf derives the signalKey a Signal belongs to.
func keyOf(sig Signal) signalKey {
	return signalKey{sig.RuleID, normalizePath(sig.Path), sig.Subject, normalizeEvidence(sig.Evidence)}
}

// groupAndOrder groups signals by keyOf and, within each group, sorts a
// copy of the signals by (Location.StartRow, Location.StartCol,
// Location.StartByte) ascending -- the index after sorting is the
// occurrence ordinal for that signal within its key. signals is not
// mutated.
func groupAndOrder(signals []Signal) map[signalKey][]Signal {
	groups := make(map[signalKey][]Signal)
	for _, sig := range signals {
		k := keyOf(sig)
		groups[k] = append(groups[k], sig)
	}

	for k, group := range groups {
		sorted := make([]Signal, len(group))
		copy(sorted, group)
		sort.SliceStable(sorted, func(i, j int) bool {
			a, b := sorted[i].Location, sorted[j].Location
			if a.StartRow != b.StartRow {
				return a.StartRow < b.StartRow
			}
			if a.StartCol != b.StartCol {
				return a.StartCol < b.StartCol
			}
			return a.StartByte < b.StartByte
		})
		groups[k] = sorted
	}

	return groups
}

// sortedKeys returns the keys of groups sorted by their field values, so
// callers iterating a map get a deterministic order.
func sortedKeys(groups map[signalKey][]Signal) []signalKey {
	keys := make([]signalKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, b := keys[i], keys[j]
		if a.ruleID != b.ruleID {
			return a.ruleID < b.ruleID
		}
		if a.path != b.path {
			return a.path < b.path
		}
		if a.subject != b.subject {
			return a.subject < b.subject
		}
		return a.evidence < b.evidence
	})
	return keys
}

// classifyFileSignals computes Fingerprint, ID, and Lifecycle for every
// signal derived from one FileChange, and returns the final signal set
// for that file: every head signal (lifecycle-classified) plus any
// base-only signal that has no matching head occurrence (lifecycle
// "resolved").
//
// hasBase is true iff the FileChange's Base result was present at all
// (fc.Base != nil), regardless of its ParseStatus -- a present Base with a
// non-"ok" ParseStatus still means "base results are not absent", it just
// means baseSignals is empty for lifecycle purposes.
func classifyFileSignals(hasBase bool, headSignals, baseSignals []Signal) []Signal {
	headGroups := groupAndOrder(headSignals)
	baseGroups := groupAndOrder(baseSignals)

	var result []Signal

	for _, k := range sortedKeys(headGroups) {
		headGroup := headGroups[k]
		baseGroup := baseGroups[k]
		nb := len(baseGroup)
		for i, sig := range headGroup {
			switch {
			case !hasBase:
				sig.Lifecycle = "unknown"
			case i < nb:
				sig.Lifecycle = "existing"
			case nb == 0:
				sig.Lifecycle = "introduced"
			default:
				sig.Lifecycle = "unknown"
			}
			sig.Fingerprint = computeFingerprint(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, i)
			sig.ID = computeSignalID(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, sig.Location.StartRow, sig.Location.StartCol, i)
			result = append(result, sig)
		}
	}

	for _, k := range sortedKeys(baseGroups) {
		baseGroup := baseGroups[k]
		headGroup := headGroups[k]
		nh := len(headGroup)
		for i, sig := range baseGroup {
			if i >= nh {
				sig.Lifecycle = "resolved"
				sig.Fingerprint = computeFingerprint(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, i)
				sig.ID = computeSignalID(sig.RuleID, sig.Path, sig.Subject, sig.Evidence, sig.Location.StartRow, sig.Location.StartCol, i)
				result = append(result, sig)
			}
		}
	}

	return result
}
