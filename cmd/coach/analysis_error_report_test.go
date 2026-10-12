package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func TestAnalysisErrorReportForRuntimeUnresolvedError(t *testing.T) {
	err := &tstoolchain.RuntimeUnresolvedError{Code: projectreadiness.GapNodeMissing, ConfigPath: "project.json"}
	got := analysisErrorReportFor(err, "typescript", false)
	if got.exitCode != 2 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).exitCode = %d, want 2", got.exitCode)
	}
	if len(got.lines) != 1 {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines = %q, want exactly the gap line", got.lines)
	}
	if got.lines[0] != err.RemediationLine() {
		t.Fatalf("analysisErrorReportFor(RuntimeUnresolvedError).lines[0] = %q, want %q", got.lines[0], err.RemediationLine())
	}
}
