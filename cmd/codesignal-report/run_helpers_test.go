package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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

func b64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
