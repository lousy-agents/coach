package render

import (
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

	got := ReportText(report)

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
			checkRenderTextLineOneBasedFrom(t, tt)
		})
	}
}

func checkRenderTextLineOneBasedFrom(t *testing.T, tt struct {
	name     string
	startRow uint
	wantLine string
}) {
	report := &codesignal.Report{
		Signals: []codesignal.Signal{{Path: "a.go", Location: semantics.Location{StartRow: tt.startRow}}},
	}

	got := ReportText(report)

	if !strings.Contains(got, tt.wantLine) {
		t.Errorf("rendered text missing %q; got:\n%s", tt.wantLine, got)
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

	got := ReportText(report)

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

	got := ReportText(report)

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

	got := ReportText(report)

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
