package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_renderPart3Test_baselineReportWithExcludedFilesShowsCoverageSect_37(t *testing.T) {
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
}

func body_renderPart3Test_nonBaselineReportWithExcludedFilesShowsCoverageS_56(t *testing.T) {
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
}

func body_renderPart3Test_nonBaselineReportWithNilCoverageOmitsCoverageSec_75(t *testing.T) {
	report := &codesignal.Report{
		Scope:   codesignal.Scope{AppliedScope: "all"},
		Summary: codesignal.Summary{FilesAnalyzed: 12, ActiveSignals: 2},
	}

	got := RenderText(report)

	if strings.Contains(got, "Coverage:") {
		t.Errorf("expected no Coverage section when nothing was filtered; got:\n%s", got)
	}
}
