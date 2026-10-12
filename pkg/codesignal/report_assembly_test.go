package codesignal

import (
	"testing"
)

func TestBuild_CoveragePassesThroughFromInputToReport(t *testing.T) {
	coverage := &Coverage{
		TrackedFilesDiscovered: 10,
		FilesAnalyzed:          8,
		FilesUnanalyzable:      2,
		Unsupported:            []CoverageGroup{{Reason: "unsupported_language", Language: "python", Count: 1}},
		Excluded:               []CoverageGroup{{Reason: "vendored", Count: 1}},
	}

	report := mustBuild(t, Options{Baseline: true}, Input{Coverage: coverage})

	if report.Coverage != coverage {
		t.Errorf("Report.Coverage: got %+v, want the same *Coverage passed in Input.Coverage", report.Coverage)
	}
}

func TestBuild_NilCoverageYieldsNilReportCoverage(t *testing.T) {
	report := mustBuild(t, Options{}, Input{})

	if report.Coverage != nil {
		t.Errorf("Report.Coverage: got %+v, want nil", report.Coverage)
	}
}
