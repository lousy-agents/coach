package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_IncludeResolvedFalseHidesResolvedButCountsThem(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
			{Kind: "mutates_input", Name: "GoneNow", Location: semantics.Location{StartRow: 2}},
		},
	}
	head := &semantics.Result{
		Path:        "f.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "f.go", Status: "modified", Base: base, Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1 (resolved signal hidden): %+v", len(report.Signals), report.Signals)
	}
	for _, s := range report.Signals {
		if s.Lifecycle == "resolved" {
			t.Errorf("Report.Signals must not contain resolved signals when IncludeResolved is false: %+v", s)
		}
	}
	if report.Summary.ResolvedSignals != 1 {
		t.Errorf("Summary.ResolvedSignals: got %d, want 1", report.Summary.ResolvedSignals)
	}
	if report.Summary.ActiveSignals != len(report.Signals) {
		t.Errorf("Summary.ActiveSignals: got %d, want %d (== len(Report.Signals))", report.Summary.ActiveSignals, len(report.Signals))
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

func TestSortSignals_AdvisorySeverityRanksAboveLow(t *testing.T) {
	if got, low := severityRank("advisory"), severityRank("low"); got <= low {
		t.Fatalf("severityRank(%q) = %d, must be strictly greater than severityRank(%q) = %d (issue #259)", "advisory", got, "low", low)
	}

	advisory := sortableSignal("a", "r", "f.go", "existing", false, "advisory", "medium", 0, 0)
	low := sortableSignal("b", "r", "f.go", "existing", false, "low", "medium", 0, 0)

	signals := []Signal{low, advisory}
	sortSignals(signals)

	if signals[0].ID != "a" || signals[1].ID != "b" {
		t.Errorf("advisory severity must sort ahead of low: got order %q,%q, want %q,%q", signals[0].ID, signals[1].ID, "a", "b")
	}
}
