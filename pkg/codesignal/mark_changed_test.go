package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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

func TestChangedRangeOverlap_MarkChangedNeverMarksResolvedSignals(t *testing.T) {
	ranges := []LineRange{{StartRow: 0, EndRow: 100}}
	signals := []Signal{
		{Lifecycle: "resolved", Location: semantics.Location{StartRow: 5, EndRow: 5}},
		{Lifecycle: "introduced", Location: semantics.Location{StartRow: 5, EndRow: 5}},
	}

	signals = markChanged(signals, ranges)

	if signals[0].Changed {
		t.Errorf("resolved signal must never have Changed=true: %+v", signals[0])
	}
	if !signals[1].Changed {
		t.Errorf("introduced signal overlapping a valid range must have Changed=true: %+v", signals[1])
	}
}

func TestBuild_ChangedSignalsSortBeforeUnchangedWithinSameLifecycleGroup(t *testing.T) {
	head := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Unchanged", Location: semantics.Location{StartRow: 100}},
			{Kind: "mutates_input", Name: "Changed", Location: semantics.Location{StartRow: 5}},
		},
	}

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{
				Path:          "f.go",
				Status:        "added",
				Head:          head,
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 10}},
			},
		},
	})

	if len(report.Signals) != 2 {
		t.Fatalf("Report.Signals length: got %d, want 2: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Subject != "Changed" || !report.Signals[0].Changed {
		t.Errorf("Signals[0] must be the Changed=true signal: %+v", report.Signals[0])
	}
	if report.Signals[1].Subject != "Unchanged" || report.Signals[1].Changed {
		t.Errorf("Signals[1] must be the Changed=false signal: %+v", report.Signals[1])
	}
}
