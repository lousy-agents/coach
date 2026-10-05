package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_ModifiedFileWithoutBaseStaysUnknown(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Update", Location: semantics.Location{StartRow: 1}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "unknown" {
		t.Errorf("Signal.Lifecycle for a modified file with no usable base: got %q, want %q", report.Signals[0].Lifecycle, "unknown")
	}
	if report.Summary.UnknownSignals != 1 {
		t.Errorf("Summary.UnknownSignals: got %d, want 1", report.Summary.UnknownSignals)
	}
	if report.Summary.IntroducedSignals != 0 {
		t.Errorf("Summary.IntroducedSignals: got %d, want 0", report.Summary.IntroducedSignals)
	}
}

func TestBuild_MissingHeadEmitsNoResolvedSignals(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "missing_head_result") {
		t.Errorf("expected a missing_head_result diagnostic, got: %+v", report.Diagnostics)
	}
}
