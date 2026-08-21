package codesignalcli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestRenderTextSignalLabels(t *testing.T) {
	report := &codesignal.Report{
		Summary: codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 1},
		Signals: []codesignal.Signal{
			{
				Path:           "a.go",
				SourceScope:    "production",
				Location:       semantics.Location{StartRow: 4},
				Lifecycle:      codesignal.Lifecycle("introduced"),
				Changed:        true,
				Evidence:       "func Update mutates input",
				WhyItMatters:   "callers may not expect their argument to be mutated",
				Recommendation: "return a new value instead of mutating input",
			},
		},
	}

	got := RenderText(report)

	for _, want := range []string{
		"path: a.go",
		"line: 5",
		"lifecycle: introduced",
		"source_scope: production",
		"changed: true",
		"evidence: func Update mutates input",
		"why it matters: callers may not expect their argument to be mutated",
		"recommendation: return a new value instead of mutating input",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing %q; got:\n%s", want, got)
		}
	}
}

func TestRenderTextLineIsOneBasedFromStartRow(t *testing.T) {
	tests := []struct {
		name     string
		startRow uint
		wantLine string
	}{
		{name: "reports line 1 for a zero start row", startRow: 0, wantLine: "line: 1"},
		{name: "reports line 11 for a start row of ten", startRow: 10, wantLine: "line: 11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &codesignal.Report{
				Signals: []codesignal.Signal{{Path: "a.go", Location: semantics.Location{StartRow: tt.startRow}}},
			}

			got := RenderText(report)

			if !strings.Contains(got, tt.wantLine) {
				t.Errorf("rendered text missing %q; got:\n%s", tt.wantLine, got)
			}
		})
	}
}

func TestRenderTextSummaryLine(t *testing.T) {
	report := &codesignal.Report{
		Summary:     codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 2},
		Diagnostics: []codesignal.Diagnostic{{Path: "a.go", Kind: "k", Message: "m"}},
	}

	got := RenderText(report)

	for _, want := range []string{"files analyzed", "active signals", "diagnostics"} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing summary substring %q; got:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "files analyzed: 3") {
		t.Errorf("expected files analyzed count 3; got:\n%s", got)
	}
	if !strings.Contains(got, "active signals: 2") {
		t.Errorf("expected active signals count 2; got:\n%s", got)
	}
	if !strings.Contains(got, "diagnostics: 1") {
		t.Errorf("expected diagnostics count 1; got:\n%s", got)
	}
}

func TestRenderTextSummaryLineScopeDisclosure(t *testing.T) {
	t.Run("discloses the filtered count under production scope", func(t *testing.T) {
		report := &codesignal.Report{
			Scope:   codesignal.Scope{AppliedScope: "production"},
			Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
			Coverage: &codesignal.Coverage{
				Excluded: []codesignal.CoverageGroup{{Reason: SourceScopeTestOnly, Language: "go", Count: 2}},
			},
		}

		got := RenderText(report)

		if !strings.Contains(got, "scope: production") {
			t.Errorf("expected summary to disclose applied scope; got:\n%s", got)
		}
		if !strings.Contains(got, "filtered: 2") {
			t.Errorf("expected summary to disclose filtered count 2; got:\n%s", got)
		}
	})

	t.Run("discloses zero filtered without claiming no scope was applied", func(t *testing.T) {
		report := &codesignal.Report{
			Scope:   codesignal.Scope{AppliedScope: "production"},
			Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
		}

		got := RenderText(report)

		if !strings.Contains(got, "scope: production") {
			t.Errorf("expected summary to disclose applied scope; got:\n%s", got)
		}
		if !strings.Contains(got, "filtered: 0") {
			t.Errorf("expected summary to disclose filtered count 0; got:\n%s", got)
		}
		if strings.Contains(got, "no scope filtering") {
			t.Errorf("zero filtered files must not read as if no scope was applied; got:\n%s", got)
		}
	})

	t.Run("all scope discloses no filtering applied", func(t *testing.T) {
		report := &codesignal.Report{
			Scope:   codesignal.Scope{AppliedScope: "all"},
			Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
		}

		got := RenderText(report)

		if !strings.Contains(got, "scope: all") {
			t.Errorf("expected summary to disclose applied scope; got:\n%s", got)
		}
		if !strings.Contains(got, "no scope filtering applied") {
			t.Errorf("expected summary to state no scope filtering was applied; got:\n%s", got)
		}
		if strings.Contains(got, "filtered:") {
			t.Errorf("scope=all must not show a filtered count as if scope were active; got:\n%s", got)
		}
	})

	t.Run("unset applied scope keeps original summary line unchanged", func(t *testing.T) {
		report := &codesignal.Report{
			Summary:     codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 2},
			Diagnostics: []codesignal.Diagnostic{{Path: "a.go", Kind: "k", Message: "m"}},
		}

		got := RenderText(report)

		wantLine := "files analyzed: 3, active signals: 2, diagnostics: 1\n"
		if !strings.HasPrefix(got, wantLine) {
			t.Errorf("expected original summary line format unchanged as first line; got:\n%s", got)
		}
		if strings.Contains(got, "scope:") {
			t.Errorf("expected no scope clause when AppliedScope is unset; got:\n%s", got)
		}
	})
}

func TestRenderTextCoverageSection(t *testing.T) {
	t.Run("baseline report with excluded files shows Coverage section", func(t *testing.T) {
		report := &codesignal.Report{
			Scope:   codesignal.Scope{Baseline: true, Revision: "abc123"},
			Summary: codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 0},
			Coverage: &codesignal.Coverage{
				Excluded: []codesignal.CoverageGroup{{Reason: SourceScopeTestOnly, Language: "go", Count: 2}},
			},
		}

		got := RenderText(report)

		if !strings.Contains(got, "Coverage:") {
			t.Errorf("expected Coverage section for baseline report with excluded files; got:\n%s", got)
		}
		if !strings.Contains(got, "excluded: 2 test_only go files") {
			t.Errorf("expected excluded coverage line; got:\n%s", got)
		}
	})

	t.Run("non-baseline report with excluded files shows Coverage section", func(t *testing.T) {
		report := &codesignal.Report{
			Scope:   codesignal.Scope{AppliedScope: "production"},
			Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
			Coverage: &codesignal.Coverage{
				Excluded: []codesignal.CoverageGroup{{Reason: SourceScopeTestOnly, Language: "go", Count: 2}},
			},
		}

		got := RenderText(report)

		if !strings.Contains(got, "Coverage:") {
			t.Errorf("expected Coverage section for non-baseline (diff) report with excluded files; got:\n%s", got)
		}
		if !strings.Contains(got, "excluded: 2 test_only go files") {
			t.Errorf("expected excluded coverage line; got:\n%s", got)
		}
	})

	t.Run("non-baseline report with nil Coverage omits Coverage section", func(t *testing.T) {
		report := &codesignal.Report{
			Scope:   codesignal.Scope{AppliedScope: "all"},
			Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
		}

		got := RenderText(report)

		if strings.Contains(got, "Coverage:") {
			t.Errorf("expected no Coverage section when nothing was filtered; got:\n%s", got)
		}
	})
}

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
			name: "prints the unqualified verdict when project coverage is non-nil but complete",
			report: &codesignal.Report{
				Summary:         codesignal.Summary{FilesAnalyzed: 1},
				ProjectCoverage: &projectmodel.Coverage{Phase: "final", Complete: true},
			},
			wantContains: []string{"No active CodeSignal findings.\n"},
			wantAbsent:   []string{"incomplete"},
		},
		{
			name: "names the distinct affected-path count when diagnostics are present",
			report: &codesignal.Report{
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
			// The fixture includes project_coverage_incomplete alongside
			// ProjectCoverage.Complete: false because the real Build pipeline
			// never leaves Diagnostics empty in that state (see
			// pkg/codesignal/codesignal.go's projectLifecycleState); a
			// fixture missing it would not match what Build actually emits.
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
			name: "counts one affected path when four diagnostics share it",
			report: &codesignal.Report{
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
			// project_observation_missing_primary_path is reachable even when
			// ProjectCoverage.Complete is true -- a distinct anomaly from the
			// two project-lifecycle diagnostic kinds above.
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
			got := RenderText(tt.report)
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
		})
	}
}

func TestRenderTextQualifiedVerdictPrecedesDiagnostics(t *testing.T) {
	report := &codesignal.Report{
		Diagnostics: []codesignal.Diagnostic{{Path: "a.go", Kind: "unsupported_change_type", Message: "m"}},
	}

	got := RenderText(report)

	verdictIdx := strings.Index(got, "the analysis is incomplete")
	diagnosticsIdx := strings.Index(got, "Diagnostics:")
	if verdictIdx < 0 || diagnosticsIdx < 0 {
		t.Fatalf("expected both the qualified verdict and a Diagnostics section; got:\n%s", got)
	}
	if !(verdictIdx < diagnosticsIdx) {
		t.Errorf("expected qualified verdict before Diagnostics section; got:\n%s", got)
	}
}

func TestRenderTextSignalsPresentRenderingIsPinnedExactly(t *testing.T) {
	report := &codesignal.Report{
		Summary: codesignal.Summary{FilesAnalyzed: 1, ActiveSignals: 1},
		Signals: []codesignal.Signal{
			{
				Path:           "a.go",
				SourceScope:    "production",
				Location:       semantics.Location{StartRow: 4},
				Lifecycle:      codesignal.Lifecycle("introduced"),
				Changed:        true,
				Evidence:       "func Update mutates input",
				WhyItMatters:   "callers may not expect their argument to be mutated",
				Recommendation: "return a new value instead of mutating input",
			},
		},
		Diagnostics:     []codesignal.Diagnostic{{Path: "b.go", Kind: "empty_content", Message: "empty"}},
		ProjectCoverage: &projectmodel.Coverage{Phase: "partial", Complete: false},
	}

	got := RenderText(report)

	want := "files analyzed: 1, active signals: 1, diagnostics: 1\n" +
		"path: a.go\n" +
		"line: 5\n" +
		"lifecycle: introduced\n" +
		"source_scope: production\n" +
		"changed: true\n" +
		"evidence: func Update mutates input\n" +
		"why it matters: callers may not expect their argument to be mutated\n" +
		"recommendation: return a new value instead of mutating input\n" +
		"\n" +
		"Diagnostics:\n" +
		"path: b.go, kind: empty_content, message: empty\n" +
		"\n" +
		"Project coverage: phase=partial, complete=false\n"

	if got != want {
		t.Errorf("signals-present path changed; a diagnostic and incomplete ProjectCoverage must not alter it.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderTextDiagnosticsSection(t *testing.T) {
	report := &codesignal.Report{
		Diagnostics: []codesignal.Diagnostic{
			{Path: "a.go", Kind: "syntax_errors", Message: "unexpected token"},
			{Path: "", Kind: "not_a_git_worktree", Message: "no path available"},
		},
	}

	got := RenderText(report)

	for _, want := range []string{
		"a.go", "syntax_errors", "unexpected token",
		"not_a_git_worktree", "no path available",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered text missing %q; got:\n%s", want, got)
		}
	}
}

func TestRenderTextNoANSIEscapes(t *testing.T) {
	report := &codesignal.Report{
		Summary: codesignal.Summary{FilesAnalyzed: 1, ActiveSignals: 1},
		Signals: []codesignal.Signal{
			{Path: "a.go", Location: semantics.Location{StartRow: 0}, Lifecycle: codesignal.Lifecycle("introduced")},
		},
		Diagnostics: []codesignal.Diagnostic{{Path: "b.go", Kind: "k", Message: "m"}},
	}

	got := RenderText(report)

	if strings.Contains(got, "\x1b[") {
		t.Errorf("rendered text contains ANSI escape sequence; got:\n%q", got)
	}
}

func TestRenderTextPreservesSignalOrder(t *testing.T) {
	report := &codesignal.Report{
		Signals: []codesignal.Signal{
			{Path: "c.go", Subject: "third"},
			{Path: "a.go", Subject: "first"},
			{Path: "b.go", Subject: "second"},
		},
	}

	got := RenderText(report)

	firstIdx := strings.Index(got, "path: c.go")
	secondIdx := strings.Index(got, "path: a.go")
	thirdIdx := strings.Index(got, "path: b.go")

	if firstIdx < 0 || secondIdx < 0 || thirdIdx < 0 {
		t.Fatalf("expected all three signal paths rendered; got:\n%s", got)
	}
	if !(firstIdx < secondIdx && secondIdx < thirdIdx) {
		t.Errorf("expected signal order c.go, a.go, b.go preserved; got:\n%s", got)
	}
}

func TestRenderJSONHasExactlyOneTrailingNewline(t *testing.T) {
	report := &codesignal.Report{
		SchemaVersion: "1",
		Summary:       codesignal.Summary{FilesAnalyzed: 1},
	}

	encoded, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON: %s", err)
	}

	if strings.Count(string(encoded), "\n") != 1 {
		t.Fatalf("expected exactly one trailing newline; got %q", encoded)
	}
}

func TestRenderJSONDoesNotAddFields(t *testing.T) {
	report := &codesignal.Report{
		SchemaVersion: "1",
		Summary:       codesignal.Summary{FilesAnalyzed: 1},
	}

	encoded, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON: %s", err)
	}

	var directMarshal map[string]any
	direct, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal: %s", err)
	}
	if err := json.Unmarshal(direct, &directMarshal); err != nil {
		t.Fatalf("json.Unmarshal direct: %s", err)
	}

	var rendered map[string]any
	if err := json.Unmarshal(encoded, &rendered); err != nil {
		t.Fatalf("json.Unmarshal rendered: %s", err)
	}

	if len(rendered) != len(directMarshal) {
		t.Errorf("RenderJSON produced a different field set than json.Marshal: rendered=%v direct=%v", rendered, directMarshal)
	}
	for k := range directMarshal {
		if _, ok := rendered[k]; !ok {
			t.Errorf("RenderJSON is missing field %q present in plain json.Marshal", k)
		}
	}
}
