package codesignal

import (
	"sort"
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func validateChangedRanges(fc FileChange) ([]Diagnostic, []LineRange) {
	var diagnostics []Diagnostic
	valid := make([]LineRange, 0, len(fc.ChangedRanges))

	for _, r := range fc.ChangedRanges {
		if r.StartRow > r.EndRow {
			diagnostics = append(diagnostics, Diagnostic{
				Path: fc.Path,
				Kind: "invalid_changed_range",
				Message: "changed range start_row " + strconv.FormatUint(uint64(r.StartRow), 10) +
					" is greater than end_row " + strconv.FormatUint(uint64(r.EndRow), 10),
			})
			continue
		}
		valid = append(valid, r)
	}

	return diagnostics, valid
}

func overlapsAny(loc semantics.Location, ranges []LineRange) bool {
	for _, r := range ranges {
		if loc.StartRow <= r.EndRow && r.StartRow <= loc.EndRow {
			return true
		}
	}
	return false
}

func markChanged(signals []Signal, validRanges []LineRange) {
	for i := range signals {
		if signals[i].Lifecycle == "resolved" {
			signals[i].Changed = false
			continue
		}
		signals[i].Changed = overlapsAny(signals[i].Location, validRanges)
	}
}

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

func severityRank(s Severity) int {
	switch s {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func confidenceRank(c Confidence) int {
	switch c {
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	default:
		return 0
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
