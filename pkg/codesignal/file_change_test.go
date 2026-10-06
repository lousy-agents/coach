package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_InvalidFileChangeEmitsDiagnostic(t *testing.T) {
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

	report := mustBuild(t, Options{}, input)

	if len(report.Diagnostics) != 2 {
		t.Fatalf("Report.Diagnostics length: got %d, want 2: %+v", len(report.Diagnostics), report.Diagnostics)
	}

	// Sorted by Path: "a.go" before "b.go".
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
