package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
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
			name: "keeps the unqualified verdict when the only diagnostic is the informational external-package count",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: ".", Kind: codesignal.DiagKindProjectExternalImportsNotEvaluated, Message: "2 import edges to external packages were not evaluated by layer rules"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"No active CodeSignal findings."},
			wantAbsent:   []string{"incomplete", "additional diagnostics were recorded"},
		},
		{
			name: "keeps the unqualified verdict for the informational count recorded on the base side",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: ".", Kind: "base_" + codesignal.DiagKindProjectExternalImportsNotEvaluated, Message: "1 import edge to an external package was not evaluated by layer rules", Side: "base"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"No active CodeSignal findings."},
			wantAbsent:   []string{"incomplete"},
		},
		{
			name: "still qualifies the verdict for an unknown diagnostic that sits beside the informational count",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: ".", Kind: codesignal.DiagKindProjectExternalImportsNotEvaluated, Message: "1 import edge to an external package was not evaluated by layer rules"},
					{Path: "a.ts", Kind: "some_future_diagnostic_kind", Message: "m"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"additional diagnostics were recorded",
			},
		},
		{
			name: "qualifies the verdict and says why when an import did not resolve",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "pkg/handlers/h.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"1 import did not resolve, so layer rules cannot tell whether it crosses a layer",
			},
			wantAbsent: []string{"additional diagnostics were recorded"},
		},
		{
			name: "does not call a no-match scan complete when an import did not resolve",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 2},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "pkg/handlers/h.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"},
				},
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "inspect_diagnostics"}},
				ProjectCoverage:    &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"found no configured covered match, but the analysis is incomplete",
				"1 import did not resolve, so layer rules cannot tell whether it crosses a layer",
			},
			wantAbsent: []string{"Complete scan"},
		},
		{
			name: "keeps calling a no-match scan complete when the only diagnostic is the informational count",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 2},
				Diagnostics: []codesignal.Diagnostic{
					{Path: ".", Kind: codesignal.DiagKindProjectExternalImportsNotEvaluated, Message: "m"},
				},
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "inspect_diagnostics"}},
				ProjectCoverage:    &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"Complete scan found no configured covered match."},
			wantAbsent:   []string{"incomplete"},
		},
		{
			name: "counts several unresolved imports across both sides of a diff",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"},
					{Path: "b.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"},
					{Path: "c.ts", Kind: "base_" + codesignal.DiagKindProjectImportUnresolved, Message: "m", Side: "base"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"3 imports did not resolve, so layer rules cannot tell whether they cross a layer",
			},
		},
		{
			name: "counts one unresolved import once when the same edge is unresolved on both sides of a diff",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "import of \"../x\" from a.ts did not resolve", Location: &semantics.Location{StartRow: 2}, Side: "head"},
					{Path: "a.ts", Kind: "base_" + codesignal.DiagKindProjectImportUnresolved, Message: "import of \"../x\" from a.ts did not resolve", Location: &semantics.Location{StartRow: 2}, Side: "base"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"1 import did not resolve, so layer rules cannot tell whether it crosses a layer"},
			wantAbsent:   []string{"2 imports did not resolve"},
		},
		{
			name: "counts two unresolved imports when the same specifier is unresolved on different lines of one file",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "import of \"../x\" from a.ts did not resolve", Location: &semantics.Location{StartRow: 2}},
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "import of \"../x\" from a.ts did not resolve", Location: &semantics.Location{StartRow: 7}},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"2 imports did not resolve, so layer rules cannot tell whether they cross a layer"},
		},
		{
			name: "qualifies the verdict and says why when a dynamic load has a non-literal specifier",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "pkg/handlers/h.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "m", Location: &semantics.Location{StartRow: 1}},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"No active CodeSignal findings, but the analysis is incomplete",
				"1 dynamic load with a non-literal specifier was not evaluated, so layer rules cannot tell whether it crosses a layer",
			},
			wantAbsent: []string{"additional diagnostics were recorded"},
		},
		{
			name: "does not call a no-match scan complete when a dynamic load has a non-literal specifier",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 2},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "pkg/handlers/h.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "m"},
				},
				ProjectNextActions: []codesignal.ProjectNextAction{{Kind: "inspect_diagnostics"}},
				ProjectCoverage:    &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"found no configured covered match, but the analysis is incomplete",
				"1 dynamic load with a non-literal specifier was not evaluated",
			},
			wantAbsent: []string{"Complete scan"},
		},
		{
			name: "counts one load once when it is reported on both sides of a diff and several loads separately",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "import() in a.ts takes (x)", Location: &semantics.Location{StartRow: 2}},
					{Path: "a.ts", Kind: "base_" + codesignal.DiagKindProjectImportNonLiteral, Message: "import() in a.ts takes (x)", Location: &semantics.Location{StartRow: 2}, Side: "base"},
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "require() in a.ts takes (y)", Location: &semantics.Location{StartRow: 2}},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"2 dynamic loads with a non-literal specifier were not evaluated, so layer rules cannot tell whether they cross a layer"},
			wantAbsent:   []string{"3 dynamic loads"},
		},
		{
			name: "counts two identical non-literal loads on one line as two, because only the two sides of a diff are deduplicated",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "require() in a.ts takes (name)", Location: &semantics.Location{StartRow: 0}},
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "require() in a.ts takes (name)", Location: &semantics.Location{StartRow: 0}},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"2 dynamic loads with a non-literal specifier were not evaluated"},
			wantAbsent:   []string{"1 dynamic load with"},
		},
		{
			name: "counts two identical unresolved imports on one line as two",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "import of \"../x\" from a.ts did not resolve", Location: &semantics.Location{StartRow: 4}},
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "import of \"../x\" from a.ts did not resolve", Location: &semantics.Location{StartRow: 4}},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"2 imports did not resolve"},
		},
		{
			name: "states both causes when an import did not resolve and a load is non-literal",
			report: &codesignal.Report{
				Summary: codesignal.Summary{FilesAnalyzed: 1},
				Diagnostics: []codesignal.Diagnostic{
					{Path: "a.ts", Kind: codesignal.DiagKindProjectImportUnresolved, Message: "m"},
					{Path: "b.ts", Kind: codesignal.DiagKindProjectImportNonLiteral, Message: "m"},
				},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{
				"1 import did not resolve, so layer rules cannot tell whether it crosses a layer; 1 dynamic load with a non-literal specifier was not evaluated",
			},
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
			checkRenderTextNoActiveFindingsVerdict(t, tt)
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

func checkRenderTextNoActiveFindingsVerdict(t *testing.T, tt struct {
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
