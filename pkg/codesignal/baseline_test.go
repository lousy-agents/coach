package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_BaselineOptionProducesBaselineLifecycle(t *testing.T) {
	b, err := New(Options{Baseline: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "service.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "ApplyDefaults", Location: semantics.Location{StartRow: 10}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "service.go", Status: "added", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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

func TestBuild_CoveragePassesThroughFromInputToReport(t *testing.T) {
	b, err := New(Options{Baseline: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	coverage := &Coverage{
		TrackedFilesDiscovered: 10,
		FilesAnalyzed:          8,
		FilesUnanalyzable:      2,
		Unsupported:            []CoverageGroup{{Reason: "unsupported_language", Language: "python", Count: 1}},
		Excluded:               []CoverageGroup{{Reason: "vendored", Count: 1}},
	}

	report, err := b.Build(context.Background(), Input{Coverage: coverage})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if report.Coverage != coverage {
		t.Errorf("Report.Coverage: got %+v, want the same *Coverage passed in Input.Coverage", report.Coverage)
	}
}

func TestBuild_NilCoverageYieldsNilReportCoverage(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	report, err := b.Build(context.Background(), Input{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if report.Coverage != nil {
		t.Errorf("Report.Coverage: got %+v, want nil", report.Coverage)
	}
}
