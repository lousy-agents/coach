package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestDiagnostics_SyntaxErrorsProduceOneDiagnosticPerIssue(t *testing.T) {
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

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "broken.go", Status: "modified", Head: head},
		},
	})

	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals for a syntax_errors head: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}

	syntaxDiagnostics := diagnosticsOfKind(report.Diagnostics, "syntax_errors")
	if len(syntaxDiagnostics) != len(issues) {
		t.Fatalf("syntax_errors diagnostics: got %d, want %d: %+v", len(syntaxDiagnostics), len(issues), syntaxDiagnostics)
	}

	for _, issue := range issues {
		if !wellFormedSyntaxDiagnosticAt(t, syntaxDiagnostics, issue.Location) {
			t.Errorf("no syntax_errors diagnostic found with Location matching issue %+v; got diagnostics %+v", issue, syntaxDiagnostics)
		}
	}
}

// wellFormedSyntaxDiagnosticAt fails t for any syntax diagnostic that lacks
// broken.go's path, a message, or a location, and reports whether one of
// them sits at location.
func wellFormedSyntaxDiagnosticAt(t *testing.T, diagnostics []Diagnostic, location semantics.Location) bool {
	t.Helper()
	found := false
	for _, d := range diagnostics {
		if d.Path != "broken.go" {
			t.Errorf("Diagnostic.Path: got %q, want %q", d.Path, "broken.go")
		}
		if d.Message == "" {
			t.Errorf("Diagnostic.Message must not be empty for %+v", d)
		}
		if d.Location == nil {
			t.Fatalf("Diagnostic.Location must not be nil for a syntax_errors diagnostic: %+v", d)
		}
		if *d.Location == location {
			found = true
		}
	}
	return found
}

func diagnosticsOfKind(diagnostics []Diagnostic, kind string) []Diagnostic {
	var matching []Diagnostic
	for _, d := range diagnostics {
		if d.Kind == kind {
			matching = append(matching, d)
		}
	}
	return matching
}
