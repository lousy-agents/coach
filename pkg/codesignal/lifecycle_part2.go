package codesignal

import (
	"sort"
)

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
