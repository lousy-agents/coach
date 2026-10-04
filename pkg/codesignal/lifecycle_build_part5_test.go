package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_AddedFileInBaselineStaysBaseline(t *testing.T) {
	b, err := New(Options{Baseline: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "tracked.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Update", Location: semantics.Location{StartRow: 1}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "tracked.go", Status: "added", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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

func TestBuild_UnsupportedParseStatusEmitsNoResolvedSignals(t *testing.T) {
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
	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("weird"),
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "unsupported_parse_status") {
		t.Errorf("expected an unsupported_parse_status diagnostic, got: %+v", report.Diagnostics)
	}
}
