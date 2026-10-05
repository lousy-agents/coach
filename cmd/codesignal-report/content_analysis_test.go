package main

import (
	"strings"
	"testing"
)

func TestRun_InvalidBase64ComposesWithBuildDiagnostics(t *testing.T) {
	input := strings.Join([]string{
		`{"repository":"example/repo","revision":"abc123"}`,
		`{"path":"bad.go","language":"go","head_content":"not-valid-base64!!","base_content":"` + b64("package bad\n") + `"}`,
		``,
	}, "\n")

	report, _ := mustRun(t, input)

	if !hasDiagnostic(report.Diagnostics, "invalid_content_encoding", "bad.go") {
		t.Errorf("expected invalid_content_encoding diagnostic for bad.go, got %+v", report.Diagnostics)
	}
	if !hasDiagnostic(report.Diagnostics, "missing_head_result", "bad.go") {
		t.Errorf("expected missing_head_result diagnostic for bad.go (from codesignal.Build), got %+v", report.Diagnostics)
	}
	if report.Summary.FilesAnalyzed != 1 {
		t.Errorf("FilesAnalyzed = %d, want 1", report.Summary.FilesAnalyzed)
	}
}

func TestRun_BinaryHeadContentProducesAnalysisFailedDiagnostic(t *testing.T) {
	binary := string([]byte{0x00, 0x01, 0x02, 'a', 'b', 'c'})
	input := strings.Join([]string{
		`{"repository":"example/repo","revision":"abc123"}`,
		`{"path":"bin.go","language":"go","head_content":"` + b64(binary) + `"}`,
		``,
	}, "\n")

	report, _ := mustRun(t, input)

	if !hasDiagnostic(report.Diagnostics, "analysis_failed", "bin.go") {
		t.Errorf("expected analysis_failed diagnostic for bin.go, got %+v", report.Diagnostics)
	}
	if len(report.Signals) != 0 {
		t.Errorf("expected no signals for a file with failed analysis, got %+v", report.Signals)
	}
}
