package codesignal

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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
			return signalLocationLess(sorted[i].Location, sorted[j].Location)
		})
		groups[k] = sorted
	}

	return groups
}

func signalLocationLess(a, b semantics.Location) bool {
	if a.StartRow != b.StartRow {
		return a.StartRow < b.StartRow
	}
	if a.StartCol != b.StartCol {
		return a.StartCol < b.StartCol
	}
	return a.StartByte < b.StartByte
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
