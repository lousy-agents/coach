package jsbridge

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func analyzeRequest(content []byte) Request {
	return Request{
		ID:         7,
		Op:         OpAnalyze,
		Path:       "main.go",
		Language:   "go",
		ContentB64: base64.StdEncoding.EncodeToString(content),
	}
}

func TestHandleEchoesID(t *testing.T) {
	resp := Handle(context.Background(), analyzeRequest([]byte("package main\n")))
	if resp.ID != 7 {
		t.Fatalf("ID = %d, want 7", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

// TestHandleTimeoutOption exercises the timeout_ms branch with a deadline
// generous enough that the analysis always completes.
func TestHandleTimeoutOption(t *testing.T) {
	req := analyzeRequest([]byte("package main\n"))
	req.TimeoutMS = 60_000
	resp := Handle(context.Background(), req)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	if resp.Result == nil || resp.Result.ParseStatus != semantics.ParseStatus("ok") {
		t.Fatalf("result = %+v, want parse_status ok", resp.Result)
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
