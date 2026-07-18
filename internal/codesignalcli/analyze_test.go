package codesignalcli

import (
	"context"
	"errors"
	"reflect"
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

	report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "", nil)
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

// TestAnalyzeChangesThreadsScopeAndCoverage verifies that AnalyzeChanges
// propagates its appliedScope and excluded parameters into the returned
// Report: a non-empty appliedScope becomes report.Scope.AppliedScope, a
// non-empty excluded becomes report.Coverage.Excluded, and an empty/nil
// excluded leaves report.Coverage nil (rather than a non-nil Coverage with an
// empty Excluded slice).
func TestAnalyzeChangesThreadsScopeAndCoverage(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "healthy.go", "package healthy\n")
	headSHA := commitFileT(t, dir, "healthy.go", "package healthy\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []SelectedFile{
		{Path: "healthy.go", Status: "modified", Language: semantics.LanguageGo},
	}

	t.Run("non-empty scope and excluded", func(t *testing.T) {
		excluded := []codesignal.CoverageGroup{{Reason: "test_only", Language: "go", Count: 1}}

		report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "production", excluded)
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
	})

	t.Run("nil excluded leaves Coverage nil", func(t *testing.T) {
		report, err := AnalyzeChanges(context.Background(), dir, headSHA, initialSHA, files, nil, "", nil)
		if err != nil {
			t.Fatalf("AnalyzeChanges: unexpected error: %v", err)
		}

		if report.Coverage != nil {
			t.Errorf("report.Coverage = %#v, want nil", report.Coverage)
		}
	})
}

// TestAnalyzeBaseline verifies AnalyzeBaseline's per-file coverage
// accounting: an unreadable file yields a head_read_failed diagnostic and
// counts toward FilesUnanalyzable, a file with a syntax error still
// produces a FileChange (so Build emits its own syntax_errors diagnostic)
// but does not count toward FilesAnalyzed, and a clean file increments
// FilesAnalyzed. The resulting Report is scoped as a baseline with
// "baseline"-lifecycle signals.
func TestAnalyzeBaseline(t *testing.T) {
	dir := newTempGitRepoT(t)
	commitFileT(t, dir, "clean.go", "package clean\n\nfunc Update(input *int) { *input = 1 }\n")
	headSHA := commitFileT(t, dir, "broken.go", "package broken\n\nfunc F( {\n")

	files := []SelectedFile{
		{Path: "clean.go", Language: semantics.LanguageGo},
		{Path: "broken.go", Language: semantics.LanguageGo},
		{Path: "missing.go", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeBaseline(context.Background(), dir, headSHA, files, nil, codesignal.Coverage{TrackedFilesDiscovered: 3})
	if err != nil {
		t.Fatalf("AnalyzeBaseline: unexpected error: %v", err)
	}

	if !report.Scope.Baseline {
		t.Errorf("report.Scope.Baseline = false, want true")
	}

	foundHeadReadFailed := false
	for _, d := range report.Diagnostics {
		if d.Path == "missing.go" && d.Kind == "head_read_failed" {
			foundHeadReadFailed = true
		}
	}
	if !foundHeadReadFailed {
		t.Errorf("report.Diagnostics = %#v, want a head_read_failed diagnostic for missing.go", report.Diagnostics)
	}

	foundSyntaxErrors := false
	for _, d := range report.Diagnostics {
		if d.Path == "broken.go" && d.Kind == "syntax_errors" {
			foundSyntaxErrors = true
		}
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
		if sig.Path == "clean.go" {
			if sig.Lifecycle != "baseline" {
				t.Errorf("signal for clean.go has Lifecycle = %q, want %q", sig.Lifecycle, "baseline")
			}
			foundBaselineSignal = true
		}
	}
	if !foundBaselineSignal {
		t.Errorf("report.Signals = %#v, want a signal for clean.go", report.Signals)
	}
}

// TestAnalyzeBaselineInterleavedReadFailures is the acceptance test for
// AnalyzeBaseline's switch from one `git show` subprocess per file to a
// single streamed `git cat-file --batch` reader. It exists to catch the new
// failure mode a shared streaming reader can introduce that per-file `git
// show` subprocesses never could: a failed/missing file's response
// misaligning the stream so a later file's content is attributed to the
// wrong path. Each successful file has a distinctive function name that
// trips the hidden_input_mutation rule, so a misaligned read would show up
// as a Signal.Subject that doesn't match the path it's attached to. Missing
// files are interleaved between them (not just trailing, as in
// TestAnalyzeBaseline) so a state leak from a mid-batch failure would have
// somewhere to manifest.
func TestAnalyzeBaselineInterleavedReadFailures(t *testing.T) {
	dir := newTempGitRepoT(t)
	commitFileT(t, dir, "a.go", "package a\n\nfunc UpdateA(input *int) { *input = 1 }\n")
	commitFileT(t, dir, "b.go", "package b\n\nfunc UpdateB(input *int) { *input = 2 }\n")
	headSHA := commitFileT(t, dir, "c.go", "package c\n\nfunc UpdateC(input *int) { *input = 3 }\n")

	files := []SelectedFile{
		{Path: "a.go", Language: semantics.LanguageGo},
		{Path: "missing1.go", Language: semantics.LanguageGo},
		{Path: "b.go", Language: semantics.LanguageGo},
		{Path: "missing2.go", Language: semantics.LanguageGo},
		{Path: "c.go", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeBaseline(context.Background(), dir, headSHA, files, nil, codesignal.Coverage{TrackedFilesDiscovered: 5})
	if err != nil {
		t.Fatalf("AnalyzeBaseline: unexpected error: %v", err)
	}

	wantSubjectByPath := map[string]string{"a.go": "UpdateA:input", "b.go": "UpdateB:input", "c.go": "UpdateC:input"}
	foundSubjectByPath := map[string]string{}
	for _, sig := range report.Signals {
		if sig.RuleID == "state.hidden_input_mutation" {
			foundSubjectByPath[sig.Path] = sig.Subject
		}
	}
	for path, wantSubject := range wantSubjectByPath {
		if got := foundSubjectByPath[path]; got != wantSubject {
			t.Errorf("hidden_input_mutation signal for %q has Subject = %q, want %q (content misaligned across the batch read)", path, got, wantSubject)
		}
	}

	for _, missing := range []string{"missing1.go", "missing2.go"} {
		found := false
		for _, d := range report.Diagnostics {
			if d.Path == missing && d.Kind == "head_read_failed" {
				found = true
			}
		}
		if !found {
			t.Errorf("report.Diagnostics = %#v, want a head_read_failed diagnostic for %q", report.Diagnostics, missing)
		}
	}

	if report.Coverage == nil {
		t.Fatal("report.Coverage = nil, want non-nil")
	}
	if report.Coverage.FilesAnalyzed != 3 {
		t.Errorf("report.Coverage.FilesAnalyzed = %d, want 3 (a.go, b.go, c.go)", report.Coverage.FilesAnalyzed)
	}
	if report.Coverage.FilesUnanalyzable != 2 {
		t.Errorf("report.Coverage.FilesUnanalyzable = %d, want 2 (missing1.go, missing2.go)", report.Coverage.FilesUnanalyzable)
	}
}

// TestAnalyzeChangesBaseReadFailureForModifiedFile verifies that a "modified"
// SelectedFile whose base content cannot be read (Git already told us the
// path existed at both revisions, so this always indicates a real read
// problem) produces a base_read_failed diagnostic and an unknown-lifecycle
// head result, rather than being silently treated as if the file had no
// base content.
func TestAnalyzeChangesBaseReadFailureForModifiedFile(t *testing.T) {
	dir := newTempGitRepoT(t)
	emptySHA := commitFileT(t, dir, "placeholder.go", "package placeholder\n")
	headSHA := commitFileT(t, dir, "a.go", "package a\n\nfunc Update(input *int) { *input = 1 }\n")

	files := []SelectedFile{
		{Path: "a.go", Status: "modified", Language: semantics.LanguageGo},
	}

	// emptySHA predates a.go's existence, so `git show emptySHA:a.go` fails
	// even though Status claims "modified" -- simulating a base-read failure
	// for a path Git already told us existed at base.
	report, err := AnalyzeChanges(context.Background(), dir, headSHA, emptySHA, files, nil, "", nil)
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
