package semantics

import (
	"testing"
)

func body_tsResultPart2Test_34(t *testing.T, r Result) {
	t.Helper()
	if r.ParseStatus != ParseStatus("ok") {
		t.Errorf("AC-R6.1: golden TS ok Result.ParseStatus: got %q, want %q", r.ParseStatus, "ok")
	}
	if len(r.Imports) != 1 || r.Imports[0].Path != "./http" {
		t.Errorf("AC-R6.1: golden TS ok Result.Imports: got %+v, want one import with Path %q", r.Imports, "./http")
	}
	if len(r.Findings) != 1 || r.Findings[0].Kind != "tight_coupling" || r.Findings[0].Name != "HttpClient" {
		t.Errorf("AC-R6.1: golden TS ok Result.Findings: got %+v, want one tight_coupling finding named %q", r.Findings, "HttpClient")
	}
	if len(r.SyntaxErrors) != 0 {
		t.Errorf("AC-R6.1: golden TS ok Result.SyntaxErrors: got %d, want 0", len(r.SyntaxErrors))
	}
}

func body_tsResultPart2Test_60(t *testing.T, r Result) {
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
