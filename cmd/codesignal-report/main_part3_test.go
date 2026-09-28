package main

import (
	"bytes"

	"strings"
	"testing"
)

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

func TestRun_ExactlyOneReportWritten(t *testing.T) {
	input := strings.Join([]string{
		"not valid json",
		`{"path":"a.go","language":"go","head_content":"` + b64("package main\n") + `"}`,
		`{"path":"missing-language"}`,
		`{"path":"bad.go","language":"go","head_content":"not-valid-base64!!"}`,
		``,
	}, "\n")

	_, raw := mustRun(t, input)

	if count := bytes.Count(raw, []byte("schema_version")); count != 1 {
		t.Errorf("expected exactly one report (one \"schema_version\" occurrence), got %d\noutput: %s", count, raw)
	}
	if lines := bytes.Count(raw, []byte("\n")); lines != 1 {
		t.Errorf("expected exactly one output line, got %d\noutput: %s", lines, raw)
	}
}
