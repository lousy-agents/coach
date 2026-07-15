package codesignal

import "sort"

type signalKey struct {
	ruleID, path, subject, evidence string
}

func keyOf(sig Signal) signalKey {
	return signalKey{sig.RuleID, normalizePath(sig.Path), sig.Subject, normalizeEvidence(sig.Evidence)}
}

// groupAndOrder groups signals by key and sorts each group by location to
// assign occurrence ordinals.
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
// signal derived from one FileChange. noBaseLifecycle is the Lifecycle
// assigned to head-only signals when hasBase is false (e.g. "unknown" for
// a base-diff Report, "baseline" for a Repository Baseline Report).
func classifyFileSignals(hasBase bool, headSignals, baseSignals []Signal, noBaseLifecycle Lifecycle) []Signal {
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
				sig.Lifecycle = noBaseLifecycle
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
