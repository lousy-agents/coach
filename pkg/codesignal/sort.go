package codesignal

import (
	"sort"
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// validateChangedRanges splits fc.ChangedRanges into diagnostics for
// invalid ranges (StartRow > EndRow) and the remaining valid ranges. An
// invalid range is dropped from the returned valid slice so downstream
// overlap checks never see it.
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

// overlapsAny reports whether loc's row span intersects any of ranges,
// treating both as 0-based inclusive.
func overlapsAny(loc semantics.Location, ranges []LineRange) bool {
	for _, r := range ranges {
		if loc.StartRow <= r.EndRow && r.StartRow <= loc.EndRow {
			return true
		}
	}
	return false
}

// markChanged sets Changed on each signal in place (signals is this file's
// own classified slice, safe to mutate). "resolved" signals carry a Base
// location, which cannot be meaningfully compared against head-revision
// changed-line ranges, so Changed is always false for them; every other
// Lifecycle comes from a genuine head-derived location and is checked
// against validRanges.
func markChanged(signals []Signal, validRanges []LineRange) {
	for i := range signals {
		if signals[i].Lifecycle == "resolved" {
			signals[i].Changed = false
			continue
		}
		signals[i].Changed = overlapsAny(signals[i].Location, validRanges)
	}
}

// signalPriorityGroup returns the sort priority group for sig, per the
// canonical order documented on sortSignals.
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

// severityRank ranks Severity high-to-low; unrecognized/empty values rank
// lowest (0) so they sort last among descending comparisons.
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

// confidenceRank ranks Confidence high-to-low; unrecognized/empty values
// rank lowest (0) so they sort last among descending comparisons.
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

// sortSignals sorts signals in place using the report's canonical priority
// order: lifecycle/changed group first (introduced+changed, existing+changed,
// introduced+unchanged, existing+unchanged, resolved, everything else), then
// within a group by severity descending, confidence descending, Path
// ascending, Location.StartRow ascending, Location.StartCol ascending,
// RuleID ascending, ID ascending.
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
