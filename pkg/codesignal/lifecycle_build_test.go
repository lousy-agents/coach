package codesignal

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// findByLifecycleAndSubject returns the one Signal in signals matching
// subject and lifecycle, failing the test if there isn't exactly one.
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

// TestBuild_LifecycleClassificationAcrossBaseAndHead proves that a file
// with both Base and Head results produces existing/introduced/resolved
// Signals in Report.Signals for a multi-finding scenario: "Existing" is in
// both, "GoneNow" is base-only (resolved), "NewOne" is head-only
// (introduced).
func TestBuild_LifecycleClassificationAcrossBaseAndHead(t *testing.T) {
	// IncludeResolved: true so the "resolved" signal this test asserts on
	// isn't filtered out of Report.Signals -- filtering is a separate
	// concern covered by sort_test.go.
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
	// The resolved signal's Location comes from Base, since Head has no
	// occurrence of this key -- proves Base data (not a zero value) is
	// what's carried through.
	if resolved.Location.StartRow != 2 {
		t.Errorf("resolved signal Location.StartRow: got %d, want 2 (Base's finding location)", resolved.Location.StartRow)
	}
}

// TestBuild_RemovedFileEmitsResolvedSignalsFromBase proves Story 4's
// removed-file rule: a FileChange with Head == nil and a Base result
// carrying findings produces "resolved" Signals in Report.Signals.
func TestBuild_RemovedFileEmitsResolvedSignalsFromBase(t *testing.T) {
	// IncludeResolved: true so the "resolved" signal this test asserts on
	// isn't filtered out of Report.Signals -- filtering is a separate
	// concern covered by sort_test.go.
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

	// A "removed" status with a nil Head must not produce a
	// missing_head_result diagnostic (that's for "modified"/"added" only).
	for _, d := range report.Diagnostics {
		if d.Kind == "missing_head_result" {
			t.Errorf("unexpected missing_head_result diagnostic for a removed file: %+v", d)
		}
	}
}
