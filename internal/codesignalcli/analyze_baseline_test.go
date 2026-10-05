package codesignalcli

import (
	"context"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

type sigTestAnalyzeBaselineS044942786 struct {
	d codesignal.
		Diagnostic
	foundHeadReadFailed *bool
}

func (sigRecv *sigTestAnalyzeBaselineS044942786) call() {

	if sigRecv.d.Path == "missing.go" && sigRecv.d.Kind == "head_read_failed" {
		*sigRecv.foundHeadReadFailed = true
	}
}

type sigTestAnalyzeBaselineS044059074 struct {
	d codesignal.
		Diagnostic
	foundSyntaxErrors *bool
}

func (sigRecv *sigTestAnalyzeBaselineS044059074) call() {

	if sigRecv.d.Path == "broken.go" && sigRecv.d.Kind == "syntax_errors" {
		*sigRecv.foundSyntaxErrors = true
	}
}

type sigTestAnalyzeBaselineS044484681 struct {
	foundBaselineSignal *bool
	sig                 codesignal.
				Signal
	t *testing.
		T
}

func (sigRecv *sigTestAnalyzeBaselineS044484681) call() {

	if sigRecv.sig.Path == "clean.go" {
		if sigRecv.sig.Lifecycle != "baseline" {
			sigRecv.t.
				Errorf("signal for clean.go has Lifecycle = %q, want %q", sigRecv.sig.Lifecycle, "baseline")
		}
		*sigRecv.foundBaselineSignal = true
	}
}

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

	files := []gitrepo.SelectedFile{
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

type sigTestAnalyzeBaselineInterleavedReadFailuresS044185478 struct {
	foundSubjectByPath map[string]string
	sig                codesignal.
				Signal
}

func (sigRecv *sigTestAnalyzeBaselineInterleavedReadFailuresS044185478) call() {

	if sigRecv.sig.RuleID == "state.hidden_input_mutation" {
		sigRecv.foundSubjectByPath[sigRecv.sig.Path] = sigRecv.sig.Subject
	}
}

type sigTestAnalyzeBaselineInterleavedReadFailuresS044248654 struct {
	foundSubjectByPath map[string]string
	path               string
	t                  *testing.
				T
	wantSubject string
}

func (sigRecv *sigTestAnalyzeBaselineInterleavedReadFailuresS044248654) call() {

	if got := sigRecv.foundSubjectByPath[sigRecv.path]; got != sigRecv.wantSubject {
		sigRecv.t.
			Errorf("hidden_input_mutation signal for %q has Subject = %q, want %q (content misaligned across the batch read)", sigRecv.path, got, sigRecv.wantSubject)
	}
}

type sigTestAnalyzeBaselineInterleavedReadFailuresS144847592 struct {
	found   *bool
	missing string
	report  *codesignal.
		Report
}

func (sigRecv *sigTestAnalyzeBaselineInterleavedReadFailuresS144847592) call() {

	for _, d := range sigRecv.report.Diagnostics {
		if d.Path == sigRecv.missing && d.Kind == "head_read_failed" {
			*sigRecv.found = true
		}
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
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n\nfunc UpdateA(input *int) { *input = 1 }\n")
	gitfixture.CommitFile(t, dir, "b.go", "package b\n\nfunc UpdateB(input *int) { *input = 2 }\n")
	headSHA := gitfixture.CommitFile(t, dir, "c.go", "package c\n\nfunc UpdateC(input *int) { *input = 3 }\n")

	files := []gitrepo.SelectedFile{
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
