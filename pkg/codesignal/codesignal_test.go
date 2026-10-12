package codesignal

import (
	"context"
	"testing"
)

func TestBuild_CleanInputProducesSchemaVersion1(t *testing.T) {
	report := mustBuild(t, Options{}, Input{})
	if report == nil {
		t.Fatalf("Build returned nil Report")
	}
	if report.SchemaVersion != "1" {
		t.Errorf("Report.SchemaVersion: got %q, want %q", report.SchemaVersion, "1")
	}
	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals: got %d, want 0", len(report.Signals))
	}
	if len(report.Diagnostics) != 0 {
		t.Errorf("Report.Diagnostics: got %d, want 0", len(report.Diagnostics))
	}
	if report.Summary.FilesAnalyzed != 0 {
		t.Errorf("Report.Summary.FilesAnalyzed: got %d, want 0", report.Summary.FilesAnalyzed)
	}
}

func TestBuild_RespectsContextCancellation(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := b.Build(ctx, Input{})
	if err == nil {
		t.Fatalf("Build with a canceled context must return an error")
	}
	if err != context.Canceled {
		t.Errorf("Build error: got %v, want %v", err, context.Canceled)
	}
	if report != nil {
		t.Errorf("Build with a canceled context must return a nil Report, got %+v", report)
	}
}
