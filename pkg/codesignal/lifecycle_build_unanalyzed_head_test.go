package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_SyntaxErrorsHeadEmitsNoResolvedSignals(t *testing.T) {
	base := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
		},
	}
	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("syntax_errors"),
		SyntaxErrors: []semantics.SyntaxIssue{
			{Kind: "error", Location: semantics.Location{StartRow: 3}},
		},
	}

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "syntax_errors") {
		t.Errorf("expected a syntax_errors diagnostic, got: %+v", report.Diagnostics)
	}
}

func TestBuild_MissingHeadEmitsNoResolvedSignals(t *testing.T) {
	base := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
		},
	}

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: nil},
		},
	})

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "missing_head_result") {
		t.Errorf("expected a missing_head_result diagnostic, got: %+v", report.Diagnostics)
	}
}

func TestBuild_UnsupportedParseStatusEmitsNoResolvedSignals(t *testing.T) {
	base := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
		},
	}
	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("weird"),
	}

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "unsupported_parse_status") {
		t.Errorf("expected an unsupported_parse_status diagnostic, got: %+v", report.Diagnostics)
	}
}

func hasDiagnosticKind(diagnostics []Diagnostic, kind string) bool {
	for _, d := range diagnostics {
		if d.Kind == kind {
			return true
		}
	}
	return false
}
