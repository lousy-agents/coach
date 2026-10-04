package semantics

import (
	"testing"
)

func body_resultPart2Test_31(t *testing.T, r Result) {
	t.Helper()
	if r.ParseStatus != ParseStatus("ok") {
		t.Errorf("AC-4.4: golden ok Result.ParseStatus: got %q, want %q", r.ParseStatus, "ok")
	}
	if len(r.Imports) != 1 {
		t.Errorf("AC-4.4: golden ok Result.Imports length: got %d, want 1", len(r.Imports))
	}
	if len(r.SyntaxErrors) != 0 {
		t.Errorf("AC-4.4: golden ok Result.SyntaxErrors: got %d, want 0", len(r.SyntaxErrors))
	}
}

func body_resultPart2Test_48(t *testing.T, r Result) {
	t.Helper()
	if r.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AC-4.4: golden syntax_errors Result.ParseStatus: got %q, want %q", r.ParseStatus, "syntax_errors")
	}
	if got, want := r.SyntaxErrors[0].Location.StartByte, uint(10); got != want {
		t.Errorf("AC-4.4: golden syntax_errors Result.SyntaxErrors[0].Location.StartByte: got %d, want %d", got, want)
	}
	if len(r.Imports) != 0 || len(r.Findings) != 0 {
		t.Errorf("AC-4.4: golden syntax_errors Result.Imports/Findings: got %d/%d, want 0/0", len(r.Imports), len(r.Findings))
	}
}
