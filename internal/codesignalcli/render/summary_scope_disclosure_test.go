package render

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/sourcescope"
	"github.com/lousy-agents/coach/pkg/codesignal"
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

func body_renderTest_disclosesTheFilteredCountUnderProductionScope_13(t *testing.T) {
	report := &codesignal.Report{
		Scope:   codesignal.Scope{AppliedScope: "production"},
		Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
		Coverage: &codesignal.Coverage{
			Excluded: []codesignal.CoverageGroup{{Reason: sourcescope.TestOnly, Language: "go", Count: 2}},
		},
	}

	got := ReportText(report)

	if !strings.Contains(got, "scope: production") {
		t.Errorf("expected summary to disclose applied scope; got:\n%s", got)
	}
	if !strings.Contains(got, "filtered: 2") {
		t.Errorf("expected summary to disclose filtered count 2; got:\n%s", got)
	}
}

func body_renderTest_disclosesZeroFilteredWithoutClaimingNoScopeWasAp_32(t *testing.T) {
	report := &codesignal.Report{
		Scope:   codesignal.Scope{AppliedScope: "production"},
		Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
	}

	got := ReportText(report)

	if !strings.Contains(got, "scope: production") {
		t.Errorf("expected summary to disclose applied scope; got:\n%s", got)
	}
	if !strings.Contains(got, "filtered: 0") {
		t.Errorf("expected summary to disclose filtered count 0; got:\n%s", got)
	}
	if strings.Contains(got, "no scope filtering") {
		t.Errorf("zero filtered files must not read as if no scope was applied; got:\n%s", got)
	}
}

func body_renderTest_allScopeDisclosesNoFilteringApplied_51(t *testing.T) {
	report := &codesignal.Report{
		Scope:   codesignal.Scope{AppliedScope: "all"},
		Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
	}

	got := ReportText(report)

	if !strings.Contains(got, "scope: all") {
		t.Errorf("expected summary to disclose applied scope; got:\n%s", got)
	}
	if !strings.Contains(got, "no scope filtering applied") {
		t.Errorf("expected summary to state no scope filtering was applied; got:\n%s", got)
	}
	if strings.Contains(got, "filtered:") {
		t.Errorf("scope=all must not show a filtered count as if scope were active; got:\n%s", got)
	}
}

func body_renderTest_unsetAppliedScopeKeepsOriginalSummaryLineUnchang_70(t *testing.T) {
	report := &codesignal.Report{
		Summary:     codesignal.Summary{FilesAnalyzed: 3, ActiveSignals: 2},
		Diagnostics: []codesignal.Diagnostic{{Path: "a.go", Kind: "k", Message: "m"}},
	}

	got := ReportText(report)

	wantLine := "files analyzed: 3, active signals: 2, diagnostics: 1\n"
	if !strings.HasPrefix(got, wantLine) {
		t.Errorf("expected original summary line format unchanged as first line; got:\n%s", got)
	}
	if strings.Contains(got, "scope:") {
		t.Errorf("expected no scope clause when AppliedScope is unset; got:\n%s", got)
	}
}
