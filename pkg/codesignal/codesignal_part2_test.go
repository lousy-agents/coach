package codesignal

import (
	"context"

	"testing"
)

func TestBuild_CleanInputProducesSchemaVersion1(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	report, err := b.Build(context.Background(), Input{})
	if err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}
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

func TestBuild_DiagnosticPathOutsideFilesCountsInSummary(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	report, err := b.Build(context.Background(), Input{
		Diagnostics: []Diagnostic{{Path: "moved.go", Kind: "unsupported_change_type", Message: "typechange"}},
	})
	if err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}
	if report.Summary.FilesWithDiagnostics != 1 {
		t.Errorf("FilesWithDiagnostics: got %d, want 1 for a diagnostic path that is not in Files", report.Summary.FilesWithDiagnostics)
	}
	if report.Summary.FilesUnanalyzed != 1 {
		t.Errorf("FilesUnanalyzed: got %d, want 1 for a diagnostic path that is not in Files", report.Summary.FilesUnanalyzed)
	}
}
