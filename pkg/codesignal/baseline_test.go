package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_BaselineOptionProducesBaselineLifecycle(t *testing.T) {
	head := &semantics.Result{
		Path:        "service.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "ApplyDefaults", Location: semantics.Location{StartRow: 10}},
		},
	}

	report := mustBuild(t, Options{Baseline: true}, Input{
		Files: []FileChange{
			{Path: "service.go", Status: "added", Head: head},
		},
	})

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "baseline" {
		t.Errorf("Signal.Lifecycle: got %q, want %q", report.Signals[0].Lifecycle, "baseline")
	}
	if report.Summary.BaselineSignals != 1 {
		t.Errorf("Report.Summary.BaselineSignals: got %d, want 1", report.Summary.BaselineSignals)
	}
	if report.Summary.IntroducedSignals != 0 {
		t.Errorf("Report.Summary.IntroducedSignals: got %d, want 0", report.Summary.IntroducedSignals)
	}
	if !report.Scope.Baseline {
		t.Errorf("Report.Scope.Baseline: got false, want true, even though caller did not set Input.Scope.Baseline")
	}
}

func TestBuild_AddedFileInBaselineStaysBaseline(t *testing.T) {
	head := &semantics.Result{
		Path:        "tracked.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Update", Location: semantics.Location{StartRow: 1}},
		},
	}

	report := mustBuild(t, Options{Baseline: true}, Input{
		Files: []FileChange{
			{Path: "tracked.go", Status: "added", Head: head},
		},
	})

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "baseline" {
		t.Errorf("Signal.Lifecycle for an added file in baseline mode: got %q, want %q", report.Signals[0].Lifecycle, "baseline")
	}
	if report.Summary.BaselineSignals != 1 {
		t.Errorf("Summary.BaselineSignals: got %d, want 1", report.Summary.BaselineSignals)
	}
	if report.Summary.IntroducedSignals != 0 {
		t.Errorf("Summary.IntroducedSignals: got %d, want 0", report.Summary.IntroducedSignals)
	}
}
