package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_ChangedSignalsSortBeforeUnchangedWithinSameLifecycleGroup(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Unchanged", Location: semantics.Location{StartRow: 100}},
			{Kind: "mutates_input", Name: "Changed", Location: semantics.Location{StartRow: 5}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{
				Path:          "f.go",
				Status:        "added",
				Head:          head,
				ChangedRanges: []LineRange{{StartRow: 0, EndRow: 10}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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

func TestSortSignals_UnrecognizedConfidenceDoesNotPanicAndSortsLast(t *testing.T) {
	bogusConfidence := sortableSignal("c", "r", "f.go", "existing", false, "medium", Confidence("bogus"), 0, 0)
	lowConfidence := sortableSignal("d", "r", "f.go", "existing", false, "medium", "low", 0, 0)

	signals2 := []Signal{bogusConfidence, lowConfidence}
	sortSignals(signals2)

	if signals2[0].ID != "d" || signals2[1].ID != "c" {
		t.Errorf("unrecognized Confidence must sort after low: got order %q,%q, want %q,%q", signals2[0].ID, signals2[1].ID, "d", "c")
	}
}

func TestSortSignals_NegativeMagnitudeStillSortsBeforeNoMagnitude(t *testing.T) {
	noMagnitude := sortableSignal("a", "r", "a.go", "existing", false, "medium", "high", 0, 0)
	negativeMagnitude := sortableSignal("b", cognitiveComplexityRule.ruleID, "z.go", "existing", false, "medium", "high", 0, 0)
	negativeMagnitude.Evidence = "cognitive_complexity=-15"

	signals := []Signal{noMagnitude, negativeMagnitude}
	sortSignals(signals)

	if signals[0].ID != "b" || signals[1].ID != "a" {
		t.Errorf("a signal with a magnitude must sort before one without, even when the ratio is negative: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "b", "a")
	}
}
