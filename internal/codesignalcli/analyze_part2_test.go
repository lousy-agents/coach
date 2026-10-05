package codesignalcli

import (
	"context"

	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// TestAnalyzeBaseline verifies AnalyzeBaseline's per-file coverage
// accounting: an unreadable file yields a head_read_failed diagnostic and
// counts toward FilesUnanalyzable, a file with a syntax error still
// produces a FileChange (so Build emits its own syntax_errors diagnostic)
// but does not count toward FilesAnalyzed, and a clean file increments
// FilesAnalyzed. The resulting Report is scoped as a baseline with
// "baseline"-lifecycle signals.
func TestAnalyzeBaseline(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "clean.go", "package clean\n\nfunc Update(input *int) { *input = 1 }\n")
	headSHA := gitfixture.CommitFile(t, dir, "broken.go", "package broken\n\nfunc F( {\n")

	files := []SelectedFile{
		{Path: "clean.go", Language: semantics.LanguageGo},
		{Path: "broken.go", Language: semantics.LanguageGo},
		{Path: "missing.go", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeBaseline(context.Background(), dir, headSHA, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 3}, nil)
	if err != nil {
		t.Fatalf("AnalyzeBaseline: unexpected error: %v", err)
	}

	if !report.Scope.Baseline {
		t.Errorf("report.Scope.Baseline = false, want true")
	}

	foundHeadReadFailed := false
	for _, d := range report.Diagnostics {
		(&sigTestAnalyzeBaselineS044942786{d: d, foundHeadReadFailed: &foundHeadReadFailed}).call()

	}
	if !foundHeadReadFailed {
		t.Errorf("report.Diagnostics = %#v, want a head_read_failed diagnostic for missing.go", report.Diagnostics)
	}

	foundSyntaxErrors := false
	for _, d := range report.Diagnostics {
		(&sigTestAnalyzeBaselineS044059074{d: d, foundSyntaxErrors: &foundSyntaxErrors}).call()

	}
	if !foundSyntaxErrors {
		t.Errorf("report.Diagnostics = %#v, want a syntax_errors diagnostic for broken.go", report.Diagnostics)
	}

	if report.Coverage == nil {
		t.Fatal("report.Coverage = nil, want non-nil")
	}
	if report.Coverage.FilesAnalyzed != 1 {
		t.Errorf("report.Coverage.FilesAnalyzed = %d, want 1 (only clean.go)", report.Coverage.FilesAnalyzed)
	}
	if report.Coverage.FilesUnanalyzable != 2 {
		t.Errorf("report.Coverage.FilesUnanalyzable = %d, want 2 (missing.go and broken.go)", report.Coverage.FilesUnanalyzable)
	}

	foundBaselineSignal := false
	for _, sig := range report.Signals {
		(&sigTestAnalyzeBaselineS044484681{foundBaselineSignal: &foundBaselineSignal, sig: sig, t: t}).call()

	}
	if !foundBaselineSignal {
		t.Errorf("report.Signals = %#v, want a signal for clean.go", report.Signals)
	}
}
