package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_RemovedFileEmitsResolvedSignalsFromBase(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "deleted.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Deleted", Location: semantics.Location{StartRow: 5}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "deleted.go", Status: "removed", Base: base, Head: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

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

func findByLifecycleAndSubject(t *testing.T, signals []Signal, subject string, lifecycle Lifecycle) Signal {
	t.Helper()
	var matches []Signal
	for _, s := range signals {
		if s.Subject == subject && s.Lifecycle == lifecycle {
			matches = append(matches, s)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("signals matching subject %q and lifecycle %q: got %d, want 1: %+v", subject, lifecycle, len(matches), signals)
	}
	return matches[0]
}
