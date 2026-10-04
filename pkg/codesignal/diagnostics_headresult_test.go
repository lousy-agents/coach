package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestDiagnostics_UnsupportedParseStatusProducesOneDiagnostic(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	head := &semantics.Result{
		Path:        "weird.go",
		Language:    semantics.LanguageGo,
		ParseStatus: semantics.ParseStatus("weird"),
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "weird.go", Status: "modified", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals for an unsupported parse status: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}

	var unsupported []Diagnostic
	for _, d := range report.Diagnostics {
		if d.Kind == "unsupported_parse_status" {
			unsupported = append(unsupported, d)
		}
	}
	if len(unsupported) != 1 {
		t.Fatalf("unsupported_parse_status diagnostics: got %d, want 1: %+v", len(unsupported), unsupported)
	}
	if unsupported[0].Path != "weird.go" {
		t.Errorf("Diagnostic.Path: got %q, want %q", unsupported[0].Path, "weird.go")
	}
	if unsupported[0].Message == "" {
		t.Errorf("Diagnostic.Message must not be empty for %+v", unsupported[0])
	}
}
