package codesignal

import (
	"cmp"
	"math"
	"slices"
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
// magnitude (signals with a magnitude first, larger ratio first), path,
// location, rule, and ID.
func sortSignals(signals []Signal) {
	slices.SortStableFunc(signals, compareSignals)
}

func compareSignals(a, b Signal) int {
	return cmp.Or(
		cmp.Compare(signalPriorityGroup(a), signalPriorityGroup(b)),
		cmp.Compare(severityRank(b.Severity), severityRank(a.Severity)),
		cmp.Compare(confidenceRank(b.Confidence), confidenceRank(a.Confidence)),
		compareMagnitudeDescending(a, b),
		cmp.Compare(a.Path, b.Path),
		cmp.Compare(a.Location.StartRow, b.Location.StartRow),
		cmp.Compare(a.Location.StartCol, b.Location.StartCol),
		cmp.Compare(a.RuleID, b.RuleID),
		cmp.Compare(a.ID, b.ID),
	)
}

// compareMagnitudeDescending orders larger magnitudes first; signals without a
// magnitude sort after every signal that has one.
func compareMagnitudeDescending(a, b Signal) int {
	return cmp.Compare(magnitudeSortKey(b), magnitudeSortKey(a))
}

func magnitudeSortKey(sig Signal) float64 {
	if ratio, ok := signalMagnitude(sig); ok {
		return ratio
	}
	return math.Inf(-1)
}
