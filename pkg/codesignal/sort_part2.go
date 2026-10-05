package codesignal

import (
	"slices"
	"strconv"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func markChanged(signals []Signal, validRanges []LineRange) []Signal {
	out := make([]Signal, len(signals))
	for i, sig := range signals {
		if sig.Lifecycle == "resolved" {
			sig.Changed = false
		} else {
			sig.Changed = overlapsAny(sig.Location, validRanges)
		}
		out[i] = sig
	}
	return out
}
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

// severityOrder is the severity vocabulary, lowest priority first.
// severityRank and ParseSeverityFloor both derive from it.
var severityOrder = []Severity{"low", "advisory", "medium", "high"}

// severityRank maps a Severity to a sort priority (higher sorts first): its
// 1-based position in severityOrder.
// "advisory" ranks above "low" (2 vs 1): it is emitted only by
// user-declared, confidence:high architecture rules (layer_violation,
// layer_bypass), so it must not sort below heuristic low-confidence
// structural findings (issue #259). It intentionally does not outrank
// "medium"/"high" -- no acceptance criterion requires that, and doing so
// would let an advisory finding eclipse a genuinely higher-severity one.
// Any severity value outside this known set -- including future additions
// not yet added to severityOrder -- ranks the same as "low" (1) rather
// than falling to a bottom bucket below every known severity; this keeps
// unknown values deterministic without silently burying them last.
func severityRank(s Severity) int {
	if i := slices.Index(severityOrder, s); i >= 0 {
		return i + 1
	}
	return 1
}
