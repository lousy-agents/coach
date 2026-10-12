package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestDiagnostics_UnsupportedParseStatusProducesOneDiagnostic(t *testing.T) {
	head := &semantics.Result{
		Path:        "weird.go",
		Language:    semantics.LanguageGo,
		ParseStatus: semantics.ParseStatus("weird"),
	}

	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "weird.go", Status: "modified", Head: head},
		},
	})

	if len(report.Signals) != 0 {
		t.Errorf("Report.Signals for an unsupported parse status: got %d, want 0: %+v", len(report.Signals), report.Signals)
	}

	unsupported := diagnosticsOfKind(report.Diagnostics, "unsupported_parse_status")
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

func TestDiagnostics_MissingHeadResultOnModifiedOrAdded(t *testing.T) {
	for _, status := range []ChangeStatus{"modified", "added"} {
		t.Run(string(status), func(t *testing.T) {
			expectOneMissingHeadDiagnostic(t, status)
		})
	}
}

func expectOneMissingHeadDiagnostic(t *testing.T, status ChangeStatus) {
	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "new.go", Status: status, Head: nil},
		},
	})

	missing := diagnosticsOfKind(report.Diagnostics, "missing_head_result")
	if len(missing) != 1 {
		t.Fatalf("missing_head_result diagnostics for status %q: got %d, want 1: %+v", status, len(missing), missing)
	}
	if missing[0].Path != "new.go" {
		t.Errorf("Diagnostic.Path: got %q, want %q", missing[0].Path, "new.go")
	}
}

func TestDiagnostics_NoMissingHeadResultWhenNotModifiedOrAdded(t *testing.T) {
	for _, status := range []ChangeStatus{"removed", "unknown", ""} {
		t.Run("status="+string(status), func(t *testing.T) {
			expectNoMissingHeadDiagnostic(t, status)
		})
	}
}

func expectNoMissingHeadDiagnostic(t *testing.T, status ChangeStatus) {
	report := mustBuild(t, Options{}, Input{
		Files: []FileChange{
			{Path: "gone.go", Status: status, Head: nil},
		},
	})

	for _, d := range report.Diagnostics {
		if d.Kind == "missing_head_result" {
			t.Errorf("unexpected missing_head_result diagnostic for status %q: %+v", status, d)
		}
	}
}
