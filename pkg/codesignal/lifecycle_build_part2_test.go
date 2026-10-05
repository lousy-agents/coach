package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_LifecycleClassificationAcrossBaseAndHead(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
			{Kind: "mutates_input", Name: "GoneNow", Location: semantics.Location{StartRow: 2}},
		},
	}
	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}},
			{Kind: "mutates_input", Name: "NewOne", Location: semantics.Location{StartRow: 20}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 3 {
		t.Fatalf("Report.Signals length: got %d, want 3: %+v", len(report.Signals), report.Signals)
	}

	existing := findByLifecycleAndSubject(t, report.Signals, "Existing", "existing")
	if existing.Fingerprint == "" || existing.ID == "" {
		t.Errorf("existing signal must have non-empty Fingerprint and ID: %+v", existing)
	}

	introduced := findByLifecycleAndSubject(t, report.Signals, "NewOne", "introduced")
	if introduced.Fingerprint == "" || introduced.ID == "" {
		t.Errorf("introduced signal must have non-empty Fingerprint and ID: %+v", introduced)
	}

	resolved := findByLifecycleAndSubject(t, report.Signals, "GoneNow", "resolved")
	if resolved.Fingerprint == "" || resolved.ID == "" {
		t.Errorf("resolved signal must have non-empty Fingerprint and ID: %+v", resolved)
	}
	if resolved.Location.StartRow != 2 {
		t.Errorf("resolved signal Location.StartRow: got %d, want 2 (Base's finding location)", resolved.Location.StartRow)
	}
}

func TestBuild_SyntaxErrorsHeadEmitsNoResolvedSignals(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

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

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "syntax_errors") {
		t.Errorf("expected a syntax_errors diagnostic, got: %+v", report.Diagnostics)
	}
}
