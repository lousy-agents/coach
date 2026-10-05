package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestRun_EmptyStdin(t *testing.T) {
	report, _ := mustRun(t, "")

	if !hasDiagnostic(report.Diagnostics, "malformed_scope_header", "") {
		t.Errorf("expected malformed_scope_header diagnostic, got %+v", report.Diagnostics)
	}
	if report.Summary.FilesAnalyzed != 0 {
		t.Errorf("FilesAnalyzed = %d, want 0", report.Summary.FilesAnalyzed)
	}
}

func TestRun_MalformedScopeHeaderDoesNotPoisonSubsequentLines(t *testing.T) {
	for _, header := range []string{"not valid json", `["a","b"]`} {
		t.Run(header, func(t *testing.T) {
			expectMalformedHeaderStillAnalyzesFiles(t, header)
		})
	}
}

func expectMalformedHeaderStillAnalyzesFiles(t *testing.T, header string) {
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

func TestRun_MalformedFileRequestLineDoesNotAbortStream(t *testing.T) {
	input := strings.Join([]string{
		`{"repository":"example/repo","revision":"abc123"}`,
		`{"path":"a.go","language":"go","head_content":"` + b64("package main\n") + `"}`,
		`{"path":"missing-language"}`,
		`{"path":"b.go","language":"go","head_content":"` + b64("package b\n") + `"}`,
		``,
	}, "\n")

	report, _ := mustRun(t, input)

	if !hasDiagnostic(report.Diagnostics, "malformed_file_request", "") {
		t.Errorf("expected malformed_file_request diagnostic with empty path, got %+v", report.Diagnostics)
	}
	if report.Summary.FilesAnalyzed != 2 {
		t.Errorf("FilesAnalyzed = %d, want 2", report.Summary.FilesAnalyzed)
	}
}
