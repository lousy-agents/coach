package codesignal

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// Diagnostic reports a problem observed while building a Report.
type Diagnostic struct {
	Path     string              `json:"path"`
	Kind     string              `json:"kind"`
	Message  string              `json:"message"`
	Location *semantics.Location `json:"location,omitempty"`
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]

		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if cmp := compareLocationPtr(a.Location, b.Location); cmp != 0 {
			return cmp < 0
		}
		return a.Message < b.Message
	})
}

func compareLocationPtr(a, b *semantics.Location) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	switch {
	case a.StartRow != b.StartRow:
		return compareUint(a.StartRow, b.StartRow)
	case a.StartCol != b.StartCol:
		return compareUint(a.StartCol, b.StartCol)
	case a.EndRow != b.EndRow:
		return compareUint(a.EndRow, b.EndRow)
	case a.EndCol != b.EndCol:
		return compareUint(a.EndCol, b.EndCol)
	case a.StartByte != b.StartByte:
		return compareUint(a.StartByte, b.StartByte)
	case a.EndByte != b.EndByte:
		return compareUint(a.EndByte, b.EndByte)
	default:
		return 0
	}
}

func compareUint(a, b uint) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}
