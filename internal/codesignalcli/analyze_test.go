package codesignalcli

import (
	"context"
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
