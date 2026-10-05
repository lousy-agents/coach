package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func body_mainTest_17(t *testing.T, header string) {
	input := strings.Join([]string{
		header,
		`{"path":"a.go","language":"go","head_content":"` + b64("package main\n") + `"}`,
		`{"path":"b.go","language":"go","head_content":"` + b64("package b\n") + `"}`,
		``,
	}, "\n")

	report, _ := mustRun(t, input)

	if !hasDiagnostic(report.Diagnostics, "malformed_scope_header", "") {
		t.Errorf("expected malformed_scope_header diagnostic, got %+v", report.Diagnostics)
	}
	if report.Scope != (codesignal.Scope{}) {
		t.Errorf("Scope = %+v, want zero value", report.Scope)
	}
	if report.Summary.FilesAnalyzed != 2 {
		t.Errorf("FilesAnalyzed = %d, want 2", report.Summary.FilesAnalyzed)
	}
}
