package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func findByLifecycleAndSubject(t *testing.T, signals []Signal, subject string, lifecycle Lifecycle) Signal {
	t.Helper()
	var matches []Signal
	for _, s := range signals {
		if s.Subject == subject && s.Lifecycle == lifecycle {
			matches = append(matches, s)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("signals matching subject %q and lifecycle %q: got %d, want 1: %+v", subject, lifecycle, len(matches), signals)
	}
	return matches[0]
}

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

func TestBuild_RemovedFileEmitsResolvedSignalsFromBase(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "deleted.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Deleted", Location: semantics.Location{StartRow: 5}},
		},
	}

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "deleted.go", Status: "removed", Base: base, Head: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "resolved" {
		t.Errorf("Signal.Lifecycle for a removed file: got %q, want %q", report.Signals[0].Lifecycle, "resolved")
	}
	if report.Signals[0].Subject != "Deleted" {
		t.Errorf("Signal.Subject: got %q, want %q", report.Signals[0].Subject, "Deleted")
	}
	if report.Signals[0].Fingerprint == "" || report.Signals[0].ID == "" {
		t.Errorf("resolved signal must have non-empty Fingerprint and ID: %+v", report.Signals[0])
	}

	for _, d := range report.Diagnostics {
		if d.Kind == "missing_head_result" {
			t.Errorf("unexpected missing_head_result diagnostic for a removed file: %+v", d)
		}
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

func TestBuild_MissingHeadEmitsNoResolvedSignals(t *testing.T) {
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

	report, err := b.Build(context.Background(), Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: nil},
		},
	})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if len(report.Signals) != 0 {
		t.Fatalf("Report.Signals length: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}
	if !hasDiagnosticKind(report.Diagnostics, "missing_head_result") {
		t.Errorf("expected a missing_head_result diagnostic, got: %+v", report.Diagnostics)
	}
}

func TestBuild_UnsupportedParseStatusEmitsNoResolvedSignals(t *testing.T) {
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
		ParseStatus: semantics.ParseStatus("weird"),
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
	if !hasDiagnosticKind(report.Diagnostics, "unsupported_parse_status") {
		t.Errorf("expected an unsupported_parse_status diagnostic, got: %+v", report.Diagnostics)
	}
}

func TestBuild_MismatchedBasePathYieldsUnknownLifecycleNotDropped(t *testing.T) {
	b, err := New(Options{IncludeResolved: true})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	base := &semantics.Result{
		Path:        "wrong.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 1}},
		},
	}
	head := &semantics.Result{
		Path:        "changed.go",
		ParseStatus: semantics.ParseStatus("ok"),
		Findings: []semantics.Finding{
			{Kind: "mutates_input", Name: "Existing", Location: semantics.Location{StartRow: 10}},
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

	if len(report.Signals) != 1 {
		t.Fatalf("Report.Signals length: got %d, want 1: %+v", len(report.Signals), report.Signals)
	}
	if report.Signals[0].Lifecycle != "unknown" {
		t.Errorf("Signal.Lifecycle with a mismatched Base path: got %q, want %q", report.Signals[0].Lifecycle, "unknown")
	}
	if !hasDiagnosticKind(report.Diagnostics, "invalid_file_change") {
		t.Errorf("expected an invalid_file_change diagnostic, got: %+v", report.Diagnostics)
	}
}
