package codesignalcli

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// TestAnalyzeChangesBaseReadFailureForModifiedFile verifies that a "modified"
// gitrepo.SelectedFile whose base content cannot be read (Git already told us the
// path existed at both revisions, so this always indicates a real read
// problem) produces a base_read_failed diagnostic and an unknown-lifecycle
// head result, rather than being silently treated as if the file had no
// base content.
func TestAnalyzeChangesBaseReadFailureForModifiedFile(t *testing.T) {
	dir := gitfixture.Init(t)
	emptySHA := gitfixture.CommitFile(t, dir, "placeholder.go", "package placeholder\n")
	headSHA := gitfixture.CommitFile(t, dir, "a.go", "package a\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []gitrepo.SelectedFile{
		{Path: "a.go", Status: "modified", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, emptySHA, files, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	foundDiagnostic := false
	for _, d := range report.Diagnostics {
		if d.Path == "a.go" && d.Kind == "base_read_failed" {
			foundDiagnostic = true
		}
	}
	if !foundDiagnostic {
		t.Errorf("report.Diagnostics = %#v, want a base_read_failed diagnostic for a.go", report.Diagnostics)
	}

	for _, sig := range report.Signals {
		if sig.Path == "a.go" && sig.Lifecycle != "unknown" {
			t.Errorf("signal %#v: Lifecycle = %q, want %q", sig, sig.Lifecycle, "unknown")
		}
	}
}

func rangesEqual(a, b []codesignal.LineRange) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
