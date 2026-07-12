package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// TestDiagnostics_SyntaxErrorsProduceOneDiagnosticPerIssue proves that a
// Head with ParseStatus "syntax_errors" and multiple SyntaxErrors entries
// produces exactly one "syntax_errors" diagnostic per issue, each carrying
// that issue's Location, and no signals for the file.
func TestDiagnostics_SyntaxErrorsProduceOneDiagnosticPerIssue(t *testing.T) {
	b, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	issues := []semantics.SyntaxIssue{
		{Kind: "error", Location: semantics.Location{StartRow: 1, StartCol: 2, EndRow: 1, EndCol: 5}},
		{Kind: "missing", Location: semantics.Location{StartRow: 3, StartCol: 0, EndRow: 3, EndCol: 1}},
	}
	head := &semantics.Result{
		Path:         "broken.go",
		Language:     semantics.LanguageGo,
		ParseStatus:  semantics.ParseStatus("syntax_errors"),
		SyntaxErrors: issues,
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "broken.go", Status: "modified", Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals for a syntax_errors head: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}

	var syntaxDiagnostics []Diagnostic
	for _, d := range report.Diagnostics {
		if d.Kind == "syntax_errors" {
			syntaxDiagnostics = append(syntaxDiagnostics, d)
		}
	}
	if len(syntaxDiagnostics) != len(issues) {
		t.Fatalf("syntax_errors diagnostics: got %d, want %d: %+v", len(syntaxDiagnostics), len(issues), syntaxDiagnostics)
	}

	for _, issue := range issues {
		found := false
		for _, d := range syntaxDiagnostics {
			if d.Path != "broken.go" {
				t.Errorf("Diagnostic.Path: got %q, want %q", d.Path, "broken.go")
			}
			if d.Message == "" {
				t.Errorf("Diagnostic.Message must not be empty for %+v", d)
			}
			if d.Location == nil {
				t.Fatalf("Diagnostic.Location must not be nil for a syntax_errors diagnostic: %+v", d)
			}
			if *d.Location == issue.Location {
				found = true
			}
		}
		if !found {
			t.Errorf("no syntax_errors diagnostic found with Location matching issue %+v; got diagnostics %+v", issue, syntaxDiagnostics)
		}
	}
}

// TestDiagnostics_UnsupportedParseStatusProducesOneDiagnostic proves that a
// Head with an unrecognized ParseStatus produces exactly one
// "unsupported_parse_status" diagnostic and no signals.
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

// TestDiagnostics_MissingHeadResultOnModifiedOrAdded proves that a nil Head
// on a "modified" or "added" FileChange produces exactly one
// "missing_head_result" diagnostic with the right Path.
func TestDiagnostics_MissingHeadResultOnModifiedOrAdded(t *testing.T) {
	for _, status := range []ChangeStatus{"modified", "added"} {
		t.Run(string(status), func(t *testing.T) {
			b, err := New(Options{})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			report, err := b.Build(context.Background(), Input{
				Files: []FileChange{
					{Path: "new.go", Status: status, Head: nil},
				},
			})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}

			var missing []Diagnostic
			for _, d := range report.Diagnostics {
				if d.Kind == "missing_head_result" {
					missing = append(missing, d)
				}
			}
			if len(missing) != 1 {
				t.Fatalf("missing_head_result diagnostics for status %q: got %d, want 1: %+v", status, len(missing), missing)
			}
			if missing[0].Path != "new.go" {
				t.Errorf("Diagnostic.Path: got %q, want %q", missing[0].Path, "new.go")
			}
		})
	}
}

// TestDiagnostics_NoMissingHeadResultWhenNotModifiedOrAdded proves that a
// nil Head on a "removed", "unknown", or empty-status FileChange produces
// no "missing_head_result" diagnostic.
func TestDiagnostics_NoMissingHeadResultWhenNotModifiedOrAdded(t *testing.T) {
	for _, status := range []ChangeStatus{"removed", "unknown", ""} {
		t.Run("status="+string(status), func(t *testing.T) {
			b, err := New(Options{})
			if err != nil {
				t.Fatalf("New: %v", err)
			}

			report, err := b.Build(context.Background(), Input{
				Files: []FileChange{
					{Path: "gone.go", Status: status, Head: nil},
				},
			})
			if err != nil {
				t.Fatalf("Build: %v", err)
			}

			for _, d := range report.Diagnostics {
				if d.Kind == "missing_head_result" {
					t.Errorf("unexpected missing_head_result diagnostic for status %q: %+v", status, d)
				}
			}
		})
	}
}
