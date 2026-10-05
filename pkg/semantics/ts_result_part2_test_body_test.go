package semantics

import (
	"testing"
)

func checkTSSyntaxErrorGoldenRoundTrip(t *testing.T, r Result) {
	t.Helper()
	if r.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AC-R6.1: golden TS syntax_errors Result.ParseStatus: got %q, want %q", r.ParseStatus, "syntax_errors")
	}
	if len(r.SyntaxErrors) == 0 {
		t.Errorf("AC-R6.1: golden TS syntax_errors Result.SyntaxErrors: got empty, want at least one issue")
	}
	if len(r.Imports) != 0 || len(r.Findings) != 0 {
		t.Errorf("AC-R6.1: golden TS syntax_errors Result.Imports/Findings: got %d/%d, want 0/0", len(r.Imports), len(r.Findings))
	}
}
