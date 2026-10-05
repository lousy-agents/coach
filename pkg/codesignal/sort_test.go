package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestChangedRangeOverlap_ValidateChangedRangesSplitsInvalidFromValid(t *testing.T) {
	fc := FileChange{
		Path: "f.go",
		ChangedRanges: []LineRange{
			{StartRow: 1, EndRow: 3},
			{StartRow: 5, EndRow: 2},
			{StartRow: 10, EndRow: 10},
			{StartRow: 8, EndRow: 4},
		},
	}

	diagnostics, valid := validateChangedRanges(fc)

	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics length: got %d, want 2: %+v", len(diagnostics), diagnostics)
	}
	for _, d := range diagnostics {
		if d.Path != "f.go" {
			t.Errorf("Diagnostic.Path: got %q, want %q", d.Path, "f.go")
		}
		if d.Kind != "invalid_changed_range" {
			t.Errorf("Diagnostic.Kind: got %q, want %q", d.Kind, "invalid_changed_range")
		}
		if d.Message == "" {
			t.Errorf("Diagnostic.Message must not be empty: %+v", d)
		}
	}

	wantValid := []LineRange{
		{StartRow: 1, EndRow: 3},
		{StartRow: 10, EndRow: 10},
	}
	if len(valid) != len(wantValid) {
		t.Fatalf("valid length: got %d, want %d: %+v", len(valid), len(wantValid), valid)
	}
	for i, r := range wantValid {
		if valid[i] != r {
			t.Errorf("valid[%d]: got %+v, want %+v", i, valid[i], r)
		}
	}
}

func TestChangedRangeOverlap_OverlapsAny(t *testing.T) {
	ranges := []LineRange{{StartRow: 10, EndRow: 20}}

	tests := []struct {
		name string
		loc  semantics.Location
		want bool
	}{
		{"strictly inside", semantics.Location{StartRow: 12, EndRow: 15}, true},
		{"strictly outside before", semantics.Location{StartRow: 1, EndRow: 5}, false},
		{"strictly outside after", semantics.Location{StartRow: 25, EndRow: 30}, false},
		{"loc end row equals range start row", semantics.Location{StartRow: 5, EndRow: 10}, true},
		{"loc start row equals range end row", semantics.Location{StartRow: 20, EndRow: 25}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body_sortTest_66(t, ranges, tc)
		})
	}
}

func TestChangedRangeOverlap_MarkChangedOutsideAllRanges(t *testing.T) {
	ranges := []LineRange{{StartRow: 10, EndRow: 20}}
	signals := []Signal{
		{Lifecycle: "existing", Location: semantics.Location{StartRow: 1, EndRow: 1}},
	}

	signals = markChanged(signals, ranges)

	if signals[0].Changed {
		t.Errorf("signal outside all ranges must have Changed=false: %+v", signals[0])
	}
}

func sortableSignal(id, ruleID, path string, lifecycle Lifecycle, changed bool, severity Severity, confidence Confidence, startRow, startCol uint) Signal {
	return Signal{
		ID:         id,
		RuleID:     ruleID,
		Path:       path,
		Lifecycle:  lifecycle,
		Changed:    changed,
		Severity:   severity,
		Confidence: confidence,
		Location:   semantics.Location{StartRow: startRow, StartCol: startCol},
	}
}
