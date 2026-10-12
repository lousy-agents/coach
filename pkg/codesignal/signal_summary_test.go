package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_IncludeResolvedFalseHidesResolvedButCountsThem(t *testing.T) {
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

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "f.go", Status: "modified", Base: base, Head: head},
		},
	})

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

func TestBuild_IncludeResolvedTrueKeepsResolvedInSignals(t *testing.T) {
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

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "f.go", Status: "modified", Base: base, Head: head},
		},
	})

	if len(report.Signals) != 2 {
		t.Fatalf("Report.Signals length: got %d, want 2 (resolved signal kept): %+v", len(report.Signals), report.Signals)
	}
	if report.Summary.ResolvedSignals != 1 {
		t.Errorf("Summary.ResolvedSignals: got %d, want 1", report.Summary.ResolvedSignals)
	}
	if report.Summary.ActiveSignals != 2 {
		t.Errorf("Summary.ActiveSignals: got %d, want 2 (unfiltered count since IncludeResolved is true)", report.Summary.ActiveSignals)
	}
}

func TestBuild_DiagnosticPathOutsideFilesCountsInSummary(t *testing.T) {
	report := mustBuild(t, Options{}, Input{
		Diagnostics: []Diagnostic{{Path: "moved.go", Kind: "unsupported_change_type", Message: "typechange"}},
	})
	if report.Summary.FilesWithDiagnostics != 1 {
		t.Errorf("FilesWithDiagnostics: got %d, want 1 for a diagnostic path that is not in Files", report.Summary.FilesWithDiagnostics)
	}
	if report.Summary.FilesUnanalyzed != 1 {
		t.Errorf("FilesUnanalyzed: got %d, want 1 for a diagnostic path that is not in Files", report.Summary.FilesUnanalyzed)
	}
}
