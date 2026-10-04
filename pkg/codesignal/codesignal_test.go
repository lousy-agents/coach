package codesignal

import (
	"context"

	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_InvalidFileChangeEmitsDiagnostic(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New returned unexpected error: %v", err)
	}

	input := Input{
		Files: []FileChange{
			{
				Path: "b.go",
				Base: &semantics.Result{Path: "b-base.go"},
			},
			{
				Path: "a.go",
				Head: &semantics.Result{Path: "a-head.go", ParseStatus: semantics.ParseStatus("ok")},
			},
			{
				Path: "c.go",
				Base: &semantics.Result{Path: "c.go"},
			},
		},
	}

	report, err := b.Build(context.Background(), input)
	if err != nil {
		t.Fatalf("Build returned unexpected error: %v", err)
	}

	if len(report.Diagnostics) != 2 {
		t.Fatalf("Report.Diagnostics length: got %d, want 2: %+v", len(report.Diagnostics), report.Diagnostics)
	}

	if report.Diagnostics[0].Path != "a.go" {
		t.Errorf("Diagnostics[0].Path: got %q, want %q", report.Diagnostics[0].Path, "a.go")
	}
	if report.Diagnostics[1].Path != "b.go" {
		t.Errorf("Diagnostics[1].Path: got %q, want %q", report.Diagnostics[1].Path, "b.go")
	}
	for _, d := range report.Diagnostics {
		if d.Kind != "invalid_file_change" {
			t.Errorf("Diagnostic.Kind: got %q, want %q", d.Kind, "invalid_file_change")
		}
		if d.Message == "" {
			t.Errorf("Diagnostic.Message must not be empty for %+v", d)
		}
	}

	if report.Summary.FilesAnalyzed != 3 {
		t.Errorf("Report.Summary.FilesAnalyzed: got %d, want 3", report.Summary.FilesAnalyzed)
	}
	if report.Summary.FilesWithDiagnostics != 2 {
		t.Errorf("Report.Summary.FilesWithDiagnostics: got %d, want 2", report.Summary.FilesWithDiagnostics)
	}
}
