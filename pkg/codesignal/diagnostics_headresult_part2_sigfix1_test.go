package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

type sigTestDiagnosticsSyntaxErrorsProduceOneDiagnosticPerIssueS0 struct {
	d                 Diagnostic
	syntaxDiagnostics *[]Diagnostic
}

func (sigRecv *sigTestDiagnosticsSyntaxErrorsProduceOneDiagnosticPerIssueS0) call() {

	if sigRecv.d.Kind == "syntax_errors" {
		*sigRecv.syntaxDiagnostics = append(*sigRecv.syntaxDiagnostics, sigRecv.d)
	}
}

type sigTestDiagnosticsSyntaxErrorsProduceOneDiagnosticPerIssueS1 struct {
	found *bool
	issue semantics.
		SyntaxIssue
	syntaxDiagnostics []Diagnostic
	t                 *testing.
				T
}

func (sigRecv *sigTestDiagnosticsSyntaxErrorsProduceOneDiagnosticPerIssueS1) call() {

	for _, d := range sigRecv.syntaxDiagnostics {
		if d.Path != "broken.go" {
			sigRecv.t.
				Errorf("Diagnostic.Path: got %q, want %q", d.Path, "broken.go")
		}
		if d.Message == "" {
			sigRecv.t.
				Errorf("Diagnostic.Message must not be empty for %+v", d)
		}
		if d.Location == nil {
			sigRecv.t.
				Fatalf("Diagnostic.Location must not be nil for a syntax_errors diagnostic: %+v", d)
		}
		if *d.Location == sigRecv.issue.Location {
			*sigRecv.found = true
		}
	}
}
