package codesignal

import (
	"sort"
)

func signalPriorityGroup(sig Signal) int {
	switch sig.Lifecycle {
	case "introduced":
		if sig.Changed {
			return 0
		}
		return 2
	case "existing":
		if sig.Changed {
			return 1
		}
		return 3
	case "resolved":
		return 4
	default:
		return 5
	}
}

// sortSignals sorts signals by priority group, severity, confidence,
// path, location, rule, and ID.
func sortSignals(signals []Signal) {
	sort.SliceStable(signals, func(i, j int) bool {
		a, b := signals[i], signals[j]

		ga, gb := signalPriorityGroup(a), signalPriorityGroup(b)
		if ga != gb {
			return ga < gb
		}
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra > rb
		}
		if ra, rb := confidenceRank(a.Confidence), confidenceRank(b.Confidence); ra != rb {
			return ra > rb
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Location.StartRow != b.Location.StartRow {
			return a.Location.StartRow < b.Location.StartRow
		}
		if a.Location.StartCol != b.Location.StartCol {
			return a.Location.StartCol < b.Location.StartCol
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		return a.ID < b.ID
	})
}
