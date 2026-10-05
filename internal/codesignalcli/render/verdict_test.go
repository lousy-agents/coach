package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

func TestRenderTextNoActiveFindingsVerdict(t *testing.T) {
	tests := []struct {
		name         string
		report       *codesignal.Report
		wantContains []string
		wantAbsent   []string
	}{
		{
			name:   "prints the unqualified verdict when nothing is missing",
			report: &codesignal.Report{Summary: codesignal.Summary{FilesAnalyzed: 1}},
			wantContains: []string{
				"No active CodeSignal findings.\n",
			},
			wantAbsent: []string{"incomplete"},
		},
		{
			name: "unsupported-only dirty trees keep the all-clear and do not say analysis is incomplete",
			report: &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Kind: codesignal.DiagKindWorktreeNotClean, Message: "working tree is not clean: notes.md"},
				},
			},
			wantContains: []string{
				"No active CodeSignal findings.\n",
				"working tree is not clean: notes.md",
			},
			wantAbsent: []string{"incomplete", "not analyzed"},
		},
		{
			name: "worktree cleanliness plus provenance still avoid the incomplete headline",
			report: &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Kind: codesignal.DiagKindWorktreeNotClean, Message: "working tree is not clean: notes.md"},
					{Kind: codesignal.DiagKindWorktreeReportReflectsCommittedHEAD, Message: "report reflects committed HEAD"},
				},
			},
			wantContains: []string{
				"No active CodeSignal findings.\n",
				"working tree is not clean: notes.md",
				"report reflects committed HEAD",
			},
			wantAbsent: []string{"incomplete", "not analyzed"},
		},
		{
			name: "prints the unqualified verdict when project coverage is non-nil but complete",
			report: &codesignal.Report{
				Summary:         codesignal.Summary{FilesAnalyzed: 1},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"No active CodeSignal findings.\n"},
			wantAbsent:   []string{"incomplete"},
		},
		{
			name: "names the unanalyzed count from Summary, not every diagnostic path",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesUnanalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.go", Kind: "unsupported_change_type", Message: "m"},
				},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"1 path was not analyzed",
			},
		},
		{

			name: "states project analysis did not complete when project coverage alone is incomplete",
			report: &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Kind: "project_coverage_incomplete", Message: "project analysis coverage is incomplete; project observations may be partial"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "partial", Complete: false},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"project analysis did not complete",
			},
			wantAbsent: []string{"not analyzed"},
		},
		{
			name: "does not call an analyzed diagnostic path unanalyzed when FilesUnanalyzed is zero",
			report: &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Path: "moved.go", Kind: "syntax_errors", Message: "m"},
				},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"additional diagnostics were recorded",
			},
			wantAbsent: []string{"not analyzed"},
		},
		{
			name: "counts one affected path when four diagnostics share it",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesUnanalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.go", Kind: "base_syntax_errors", Message: "m1"},
					{Path: "a.go", Kind: "base_syntax_errors", Message: "m2"},
					{Path: "a.go", Kind: "base_syntax_errors", Message: "m3"},
					{Path: "a.go", Kind: "base_syntax_errors", Message: "m4"},
				},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"1 path was not analyzed",
			},
			wantAbsent: []string{"4 paths were not analyzed"},
		},
		{

			name: "falls back to a generic incomplete-analysis clause for a pathless diagnostic unrelated to project coverage",
			report: &codesignal.Report{
				Diagnostics: []codesignal.Diagnostic{
					{Kind: "project_observation_missing_primary_path", Message: "m"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"additional diagnostics were recorded",
			},
			wantAbsent: []string{"not analyzed", "project analysis did not complete"},
		},
		{
			name: "states both causes when diagnostics and incomplete project coverage co-occur",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesUnanalyzed: 2},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.go", Kind: "unsupported_change_type", Message: "m"},
					{Path: "b.go", Kind: "unsupported_change_type", Message: "m"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "partial", Complete: false},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"2 paths were not analyzed",
				"project analysis did not complete",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_renderPart4Test_155(t, tt)
		})
	}
}

func TestRenderTextQualifiedVerdictPrecedesDiagnostics(t *testing.T) {
	report := &codesignal.Report{
		Diagnostics: []codesignal.Diagnostic{{Path: "a.go", Kind: "unsupported_change_type", Message: "m"}},
	}

	got := ReportText(report)

	verdictIdx := strings.Index(got, "the analysis is incomplete")
	diagnosticsIdx := strings.Index(got, "Diagnostics:")
	if verdictIdx < 0 || diagnosticsIdx < 0 {
		t.Fatalf("expected both the qualified verdict and a Diagnostics section; got:\n%s", got)
	}
	if !(verdictIdx < diagnosticsIdx) {
		t.Errorf("expected qualified verdict before Diagnostics section; got:\n%s", got)
	}
}

func body_renderPart4Test_155(t *testing.T, tt struct {
	name         string
	report       *codesignal.Report
	wantContains []string
	wantAbsent   []string
}) {
	got := ReportText(tt.report)
	for _, want := range tt.wantContains {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing %q; got:\n%s", want, got)
		}
	}
	for _, absent := range tt.wantAbsent {
		if strings.Contains(got, absent) {
			t.Errorf("rendered text must not contain %q; got:\n%s", absent, got)
		}
	}
}
