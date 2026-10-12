package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestBuild_LifecycleClassificationAcrossBaseAndHead(t *testing.T) {
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

	report := mustBuild(t, Options{IncludeResolved: true}, Input{
		Files: []FileChange{
			{Path: "changed.go", Status: "modified", Base: base, Head: head},
		},
	})

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
