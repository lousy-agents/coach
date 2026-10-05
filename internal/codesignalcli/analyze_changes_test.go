package codesignalcli

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// TestAnalyzeChangesThreadsScopeAndCoverage verifies that AnalyzeChanges
// propagates its appliedScope and excluded parameters into the returned
// Report: a non-empty appliedScope becomes report.Scope.AppliedScope, a
// non-empty excluded becomes report.Coverage.Excluded, and an empty/nil
// excluded leaves report.Coverage nil (rather than a non-nil Coverage with an
// empty Excluded slice).
func TestAnalyzeChangesThreadsScopeAndCoverage(t *testing.T) {
	dir := gitfixture.Init(t)
	initialSHA := gitfixture.CommitFile(t, dir, "healthy.go", "package healthy\n")
	headSHA := gitfixture.CommitFile(t, dir, "healthy.go", "package healthy\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []gitrepo.SelectedFile{
		{Path: "healthy.go", Status: "modified", Language: semantics.LanguageGo},
	}

	t.Run("non-empty scope and excluded", func(t *testing.T) {
		body_analyzePart4Test_nonEmptyScopeAndExcluded_28(t, dir, initialSHA, headSHA, files)
	})

	t.Run("nil excluded leaves Coverage nil", func(t *testing.T) {
		body_analyzePart4Test_nilExcludedLeavesCoverageNil_47(t, dir, initialSHA, headSHA, files)
	})
}

func body_analyzePart4Test_nonEmptyScopeAndExcluded_28(t *testing.T, dir string, initialSHA string, headSHA string, files []gitrepo.SelectedFile) {
	excluded := []codesignal.CoverageGroup{{Reason: "test_only", Language: "go", Count: 1}}

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "production", excluded, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	if report.Scope.AppliedScope != "production" {
		t.Errorf("report.Scope.AppliedScope = %q, want %q", report.Scope.AppliedScope, "production")
	}
	if report.Coverage == nil {
		t.Fatal("report.Coverage = nil, want non-nil")
	}
	if !reflect.DeepEqual(report.Coverage.Excluded, excluded) {
		t.Errorf("report.Coverage.Excluded = %#v, want %#v", report.Coverage.Excluded, excluded)
	}
}

func body_analyzePart4Test_nilExcludedLeavesCoverageNil_47(t *testing.T, dir string, initialSHA string, headSHA string, files []gitrepo.SelectedFile) {
	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	if report.Coverage != nil {
		t.Errorf("report.Coverage = %#v, want nil", report.Coverage)
	}
}

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

// TestAnalyzeChangesSurvivesUnreadableFile verifies that a gitrepo.SelectedFile
// pointing at a path git show cannot read produces a diagnostic instead of
// crashing the run, and that other files in the same batch are still
// analyzed.
func TestAnalyzeChangesSurvivesUnreadableFile(t *testing.T) {
	dir := gitfixture.Init(t)
	initialSHA := gitfixture.CommitFile(t, dir, "healthy.go", "package healthy\n")
	headSHA := gitfixture.CommitFile(t, dir, "healthy.go", "package healthy\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []gitrepo.SelectedFile{
		{Path: "healthy.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "does-not-exist.go", Status: "modified", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "", nil, nil)
	if err != nil {
		t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
	}

	foundDiagnostic := false
	for _, d := range report.Diagnostics {
		if d.Path == "does-not-exist.go" {
			foundDiagnostic = true
		}
	}
	if !foundDiagnostic {
		t.Errorf("report.Diagnostics = %#v, want a diagnostic for does-not-exist.go", report.Diagnostics)
	}

	foundHealthySignal := false
	for _, sig := range report.Signals {
		if sig.Path == "healthy.go" {
			foundHealthySignal = true
		}
	}
	if !foundHealthySignal {
		t.Errorf("report.Signals = %#v, want a signal for healthy.go", report.Signals)
	}
}

func TestMapSemanticsError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "empty content", err: semantics.ErrEmptyContent, want: "empty_content"},
		{name: "binary content", err: semantics.ErrBinaryContent, want: "binary_content"},
		{name: "file too large", err: semantics.ErrFileTooLarge, want: "file_too_large"},
		{name: "unsupported language", err: semantics.ErrUnsupportedLanguage, want: "unsupported_language"},
		{name: "anything else", err: errors.New("boom"), want: "analysis_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_analyzeTest_66(t, tt)
		})
	}
}

func body_analyzeTest_66(t *testing.T, tt struct {
	name string
	err  error
	want string
}) {
	got := mapSemanticsError("some/path.go", tt.err)
	if got.Kind != tt.want {
		t.Errorf("mapSemanticsError(%v).Kind = %q, want %q", tt.err, got.Kind, tt.want)
	}
	if got.Path != "some/path.go" {
		t.Errorf("mapSemanticsError(%v).Path = %q, want %q", tt.err, got.Path, "some/path.go")
	}
	if got.Message == "" {
		t.Errorf("mapSemanticsError(%v).Message is empty, want non-empty", tt.err)
	}
}
