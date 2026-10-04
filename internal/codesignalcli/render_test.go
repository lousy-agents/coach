package codesignalcli

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestRenderTextSummaryLineScopeDisclosure(t *testing.T) {
	t.Run("discloses the filtered count under production scope", func(t *testing.T) {
		body_renderTest_disclosesTheFilteredCountUnderProductionScope_13(t)
	})

	t.Run("discloses zero filtered without claiming no scope was applied", func(t *testing.T) {
		body_renderTest_disclosesZeroFilteredWithoutClaimingNoScopeWasAp_32(t)
	})

	t.Run("all scope discloses no filtering applied", func(t *testing.T) {
		body_renderTest_allScopeDisclosesNoFilteringApplied_51(t)
	})

	t.Run("unset applied scope keeps original summary line unchanged", func(t *testing.T) {
		body_renderTest_unsetAppliedScopeKeepsOriginalSummaryLineUnchang_70(t)
	})
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
