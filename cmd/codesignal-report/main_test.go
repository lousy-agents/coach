package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func mustRun(t *testing.T, input string) (*codesignal.Report, []byte) {
	t.Helper()

	var out bytes.Buffer
	if err := run(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("run: unexpected error: %v", err)
	}

	var report codesignal.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	return &report, out.Bytes()
}

func hasDiagnostic(diagnostics []codesignal.Diagnostic, kind, path string) bool {
	for _, d := range diagnostics {
		if d.Kind == kind && d.Path == path {
			return true
		}
	}
	return false
}

func TestRun_TwoValidFileRequestsWithScopeHeader(t *testing.T) {
	input := strings.Join([]string{
		`{"repository":"example/repo","revision":"abc123","base":"main"}`,
		`{"path":"a.go","language":"go","head_content":"` + b64("package main\n") + `"}`,
		`{"path":"b.go","language":"go","head_content":"` + b64("package b\n") + `"}`,
		``,
	}, "\n")

	report, _ := mustRun(t, input)

	if report.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", report.SchemaVersion, "1")
	}
	if report.Summary.FilesAnalyzed != 2 {
		t.Errorf("FilesAnalyzed = %d, want 2", report.Summary.FilesAnalyzed)
	}
	want := codesignal.Scope{Repository: "example/repo", Revision: "abc123", Base: "main"}
	if report.Scope != want {
		t.Errorf("Scope = %+v, want %+v", report.Scope, want)
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

func TestRun_MalformedScopeHeaderDoesNotPoisonSubsequentLines(t *testing.T) {
	for _, header := range []string{"not valid json", `["a","b"]`} {
		t.Run(header, func(t *testing.T) {
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
		})
	}
}

func TestRun_EmptyStdin(t *testing.T) {
	report, _ := mustRun(t, "")

	if !hasDiagnostic(report.Diagnostics, "malformed_scope_header", "") {
		t.Errorf("expected malformed_scope_header diagnostic, got %+v", report.Diagnostics)
	}
	if report.Summary.FilesAnalyzed != 0 {
		t.Errorf("FilesAnalyzed = %d, want 0", report.Summary.FilesAnalyzed)
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

func TestGofmt(t *testing.T) {
	out, err := exec.Command("gofmt", "-l", ".").Output()
	if err != nil {
		t.Fatalf("gofmt: %v", err)
	}
	if len(bytes.TrimSpace(out)) != 0 {
		t.Errorf("gofmt -l reported unformatted files:\n%s", out)
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

func TestRun_SmokeViaGoRun(t *testing.T) {
	input := strings.Join([]string{
		`{"repository":"example/repo","revision":"abc123"}`,
		`{"path":"main.go","language":"go","head_content":"` + b64("package main\n") + `"}`,
		``,
	}, "\n")

	cmd := exec.Command("go", "run", ".")
	cmd.Stdin = strings.NewReader(input)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run .: %v\nstderr: %s", err, stderr.String())
	}

	var report codesignal.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if report.SchemaVersion != "1" {
		t.Errorf("SchemaVersion = %q, want %q", report.SchemaVersion, "1")
	}
}
