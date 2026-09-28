package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

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
		(&sigTestDiagnosticsSyntaxErrorsProduceOneDiagnosticPerIssueS0{d: d, syntaxDiagnostics: &syntaxDiagnostics}).call()

	}
	if len(syntaxDiagnostics) != len(issues) {
		t.Fatalf("syntax_errors diagnostics: got %d, want %d: %+v", len(syntaxDiagnostics), len(issues), syntaxDiagnostics)
	}

	for _, issue := range issues {
		found := false
		(&sigTestDiagnosticsSyntaxErrorsProduceOneDiagnosticPerIssueS1{found: &found, issue: issue, syntaxDiagnostics: syntaxDiagnostics, t: t}).call()

		if !found {
			t.Errorf("no syntax_errors diagnostic found with Location matching issue %+v; got diagnostics %+v", issue, syntaxDiagnostics)
		}
	}
}
