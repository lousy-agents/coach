package codesignalcli

import (
	"context"

	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

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
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n\nfunc UpdateA(input *int) { *input = 1 }\n")
	gitfixture.CommitFile(t, dir, "b.go", "package b\n\nfunc UpdateB(input *int) { *input = 2 }\n")
	headSHA := gitfixture.CommitFile(t, dir, "c.go", "package c\n\nfunc UpdateC(input *int) { *input = 3 }\n")

	files := []SelectedFile{
		{Path: "a.go", Language: semantics.LanguageGo},
		{Path: "missing1.go", Language: semantics.LanguageGo},
		{Path: "b.go", Language: semantics.LanguageGo},
		{Path: "missing2.go", Language: semantics.LanguageGo},
		{Path: "c.go", Language: semantics.LanguageGo},
	}

	report, err := AnalyzeBaseline(context.Background(), dir, headSHA, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 5}, nil)
	if err != nil {
		t.Fatalf("AnalyzeBaseline: unexpected error: %v", err)
	}

	wantSubjectByPath := map[string]string{"a.go": "UpdateA:input", "b.go": "UpdateB:input", "c.go": "UpdateC:input"}
	foundSubjectByPath := map[string]string{}
	for _, sig := range report.Signals {
		(&sigTestAnalyzeBaselineInterleavedReadFailuresS044185478{foundSubjectByPath: foundSubjectByPath, sig: sig}).call()

	}
	for path, wantSubject := range wantSubjectByPath {
		(&sigTestAnalyzeBaselineInterleavedReadFailuresS044248654{foundSubjectByPath: foundSubjectByPath, path: path, t: t, wantSubject: wantSubject}).call()

	}

	for _, missing := range []string{"missing1.go", "missing2.go"} {
		found := false
		(&sigTestAnalyzeBaselineInterleavedReadFailuresS144847592{found: &found, missing: missing, report: report}).call()

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
