package main

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestRun_SmokeViaGoRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping go run smoke test in -short mode")
	}

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
