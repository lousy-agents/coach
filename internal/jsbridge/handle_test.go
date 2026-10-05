package jsbridge

import (
	"context"
	"encoding/base64"
	"flag"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

var update = flag.Bool("update", false, "regenerate testdata/parity expected files")

// parityCase is one entry of testdata/parity/manifest.json. The manifest is
// shared with js/semantics/test/parity.test.ts, so both sides of the bridge
// replay exactly the same requests.
type parityCase struct {
	Name     string  `json:"name"`
	Src      string  `json:"src"`
	Path     string  `json:"path"`
	Language string  `json:"language"`
	Options  Options `json:"options"`
	Expected string  `json:"expected"`
}

// TestParityFixtures locks the exact Response JSON for every manifest case.
// The expected files are authoritative-by-construction: `go test -run
// TestParityFixtures ./internal/jsbridge -update` regenerates them from
// whatever Handle actually emits, and this test fails on any drift, so the
// JS parity suite always compares against Go-canonical bytes.
//
// Exception: testdata/parity/ts_syntax_errors.expected.json's exact
// syntax_errors byte positions/count are backend-implementation-defined and
// not frozen across the CGO-vs-gotreesitter transition (issue #33) -- only
// parse_status == "syntax_errors" with len(syntax_errors) >= 1 is the
// frozen contract for that one case; the checked-in bytes just pin
// gotreesitter's current actual output so drift is visible, not a spec.
func TestParityFixtures(t *testing.T) {
	for _, tc := range loadManifest(t) {
		t.Run(tc.Name, (&sigTestParityFixtures12322523{tc: tc}).call)
	}
}

// TestHandleSyntaxDoubleReturn locks the both-fields contract: a syntax
// failure yields a partial Result and an error in the same Response.
func TestHandleSyntaxDoubleReturn(t *testing.T) {
	resp := Handle(context.Background(), analyzeRequest([]byte("package main\nfunc oops( {\n")))
	if resp.Error == nil || resp.Error.Kind != KindSyntax {
		t.Fatalf("error = %+v, want kind %q", resp.Error, KindSyntax)
	}
	if resp.Result == nil {
		t.Fatal("Result is nil, want partial result alongside the syntax error")
	}
	if resp.Result.ParseStatus != semantics.ParseStatus("syntax_errors") {
		t.Fatalf("parse_status = %q, want syntax_errors", resp.Result.ParseStatus)
	}
	if len(resp.Result.SyntaxErrors) == 0 {
		t.Fatal("partial result carries no syntax_errors")
	}
}

func analyzeRequest(content []byte) Request {
	return Request{
		ID:         7,
		Op:         OpAnalyze,
		Path:       "main.go",
		Language:   "go",
		ContentB64: base64.StdEncoding.EncodeToString(content),
	}
}
