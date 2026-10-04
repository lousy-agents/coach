package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_AddedFileWithoutBaseIsIntroducedInDiffMode(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "new.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Update", Location: semantics.Location{StartRow: 1}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "new.go", Status: "added", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "introduced" {
		t.Errorf("Signal.Lifecycle for an added file in diff mode: got %q, want %q", report.Signals[0].Lifecycle, "introduced")
	}
	if report.Summary.IntroducedSignals != 1 {
		t.Errorf("Summary.IntroducedSignals: got %d, want 1", report.Summary.IntroducedSignals)
	}
	if report.Summary.UnknownSignals != 0 {
		t.Errorf("Summary.UnknownSignals: got %d, want 0", report.Summary.UnknownSignals)
	}
}

func TestBuild_MismatchedBasePathYieldsUnknownLifecycleNotDropped(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "wrong.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
		},
	}
	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "unknown" {
		t.Errorf("Signal.Lifecycle with a mismatched Base path: got %q, want %q", report.Signals[0].Lifecycle, "unknown")
	}
	if !hasDiagnosticKind(report.Diagnostics, "invalid_file_change") {
		t.Errorf("expected an invalid_file_change diagnostic, got: %+v", report.Diagnostics)
	}
}
