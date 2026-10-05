package codesignal

import (
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

func overlapsAny(loc semantics.Location, ranges []LineRange) bool {
	for _, r := range ranges {
		if loc.StartRow <= r.EndRow && r.StartRow <= loc.EndRow {
			return true
		}
	}
	return false
}
