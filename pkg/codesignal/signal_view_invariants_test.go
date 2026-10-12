package codesignal

import "testing"

func TestNarrowLeavesTheSourceReportAndFullAnalysisBlocksUntouched(t *testing.T) {
	report := reportWithSeverities("2", "high", "low")

	got := mustNarrow(t, report, NarrowOptions{MinSeverity: "high", Top: 1})

	if len(report.Signals) != 2 || len(report.ProjectChanges) != 2 || report.SignalsWithheld != nil {
		t.Fatalf("source report was mutated: %+v", report)
	}
	if got.Summary != report.Summary {
		t.Fatalf("summary = %+v, want %+v", got.Summary, report.Summary)
	}
	if got.Coverage != report.Coverage {
		t.Fatal("coverage must describe the full analysis")
	}
}

func TestNarrowKeepsAbsentProjectChangesAbsent(t *testing.T) {
	report := reportWithSeverities("2", "high", "low")
	report.ProjectChanges = nil

	got := mustNarrow(t, report, NarrowOptions{MinSeverity: "high"})

	if got.ProjectChanges != nil {
		t.Fatalf("project changes = %+v, want nil", got.ProjectChanges)
	}
}

func TestNarrowTreatsUnknownSeverityLikeLow(t *testing.T) {
	report := reportWithSeverities("1", "bogus")

	if got := mustNarrow(t, report, NarrowOptions{MinSeverity: "low"}); len(got.Signals) != 1 {
		t.Fatalf("unknown severity must survive a low floor, got %d signals", len(got.Signals))
	}
	if got := mustNarrow(t, report, NarrowOptions{MinSeverity: "advisory"}); len(got.Signals) != 0 || got.SignalsWithheld.BelowMinSeverity != 1 {
		t.Fatalf("unknown severity must be withheld above low, got %+v", got)
	}
}
