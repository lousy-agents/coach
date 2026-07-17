package codesignalcli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
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
		{name: "zero start row", startRow: 0, wantLine: "line: 1"},
		{name: "start row ten", startRow: 10, wantLine: "line: 11"},
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
	t.Run("production scope with filtered files", func(t *testing.T) {
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

	t.Run("production scope with nothing filtered", func(t *testing.T) {
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

func TestRenderTextNoActiveSignals(t *testing.T) {
	report := &codesignal.Report{Summary: codesignal.Summary{FilesAnalyzed: 1}}

	got := RenderText(report)

	if !strings.Contains(got, "No active CodeSignal findings.") {
		t.Errorf("expected exact sentence \"No active CodeSignal findings.\"; got:\n%s", got)
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

func TestRenderJSONDoesNotAddFields(t *testing.T) {
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
