package codesignalcli

import (
	"context"
	"errors"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestParseChangedRanges(t *testing.T) {
	tests := []struct {
		name    string
		diff    string
		want    []codesignal.LineRange
		wantErr bool
	}{
		{
			name: "first-line insertion",
			diff: "diff --git a/f.go b/f.go\n" +
				"--- a/f.go\n" +
				"+++ b/f.go\n" +
				"@@ -0,0 +1,3 @@\n" +
				"+a\n+b\n+c\n",
			want: []codesignal.LineRange{{StartRow: 0, EndRow: 2}},
		},
		{
			name: "multi-line replacement",
			diff: "@@ -2,2 +2,4 @@\n" +
				"-old1\n-old2\n+new1\n+new2\n+new3\n+new4\n",
			want: []codesignal.LineRange{{StartRow: 1, EndRow: 4}},
		},
		{
			name: "deletion-only hunk emits no range",
			diff: "@@ -5,3 +4,0 @@\n" +
				"-a\n-b\n-c\n",
			want: nil,
		},
		{
			name: "two disjoint hunks",
			diff: "@@ -1,1 +1,1 @@\n" +
				"-a\n+z\n" +
				"@@ -10,1 +10,2 @@\n" +
				"-j\n+k\n+l\n",
			want: []codesignal.LineRange{
				{StartRow: 0, EndRow: 0},
				{StartRow: 9, EndRow: 10},
			},
		},
		{
			name: "eof-adjacent addition",
			diff: "@@ -20,0 +21,2 @@\n" +
				"+x\n+y\n",
			want: []codesignal.LineRange{{StartRow: 20, EndRow: 21}},
		},
		{
			name: "omitted count means 1",
			diff: "@@ -1 +1 @@\n" +
				"-a\n+b\n",
			want: []codesignal.LineRange{{StartRow: 0, EndRow: 0}},
		},
		{
			name: "no hunks",
			diff: "diff --git a/f.go b/f.go\n--- a/f.go\n+++ b/f.go\n",
			want: nil,
		},
		{
			name:    "malformed hunk header",
			diff:    "@@ garbage @@\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseChangedRanges([]byte(tt.diff))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseChangedRanges(%q): want error, got nil", tt.diff)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseChangedRanges(%q): unexpected error: %v", tt.diff, err)
			}
			if !rangesEqual(got, tt.want) {
				t.Errorf("parseChangedRanges(%q) = %#v, want %#v", tt.diff, got, tt.want)
			}
		})
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
		})
	}
}

// TestAnalyzeChangesSurvivesUnreadableFile verifies that a SelectedFile
// pointing at a path git show cannot read produces a diagnostic instead of
// crashing the run, and that other files in the same batch are still
// analyzed.
func TestAnalyzeChangesSurvivesUnreadableFile(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "healthy.go", "package healthy\n")
	headSHA := commitFileT(t, dir, "healthy.go", "package healthy\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []SelectedFile{
		{Path: "healthy.go", Status: "modified", Language: semantics.LanguageGo},
		{Path: "does-not-exist.go", Status: "modified", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil)
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
