package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_RemovedFileEmitsResolvedSignalsFromBase(t *testing.T) {
	base := &semantics.Result{
		Path:        "deleted.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Deleted", Location: semantics.Location{StartRow: 5}},
		},
	}

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "deleted.go", Status: "removed", Base: base, Head: nil},
		},
	})

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "resolved" {
		t.Errorf("Signal.Lifecycle for a removed file: got %q, want %q", report.Signals[0].Lifecycle, "resolved")
	}
	if report.Signals[0].Subject != "Deleted" {
		t.Errorf("Signal.Subject: got %q, want %q", report.Signals[0].Subject, "Deleted")
	}
	if report.Signals[0].Fingerprint == "" || report.Signals[0].ID == "" {
		t.Errorf("resolved signal must have non-empty Fingerprint and ID: %+v", report.Signals[0])
	}

	for _, d := range report.Diagnostics {
		if d.Kind == "missing_head_result" {
			t.Errorf("unexpected missing_head_result diagnostic for a removed file: %+v", d)
		}
	}
}
