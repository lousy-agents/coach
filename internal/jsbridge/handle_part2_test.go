package jsbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestErrorKinds(t *testing.T) {
	tests := []struct {
		name string
		req  Request
		kind string
	}{
		{
			name: "empty content",
			req:  analyzeRequest(nil),
			kind: KindEmptyContent,
		},
		{
			name: "unsupported language",
			req: Request{
				Op:         OpAnalyze,
				Language:   "python",
				ContentB64: base64.StdEncoding.EncodeToString([]byte("print(1)\n")),
			},
			kind: KindUnsupportedLanguage,
		},
		{
			name: "language outside analyzer subset",
			req: Request{
				Op:         OpAnalyze,
				Language:   "typescript",
				ContentB64: base64.StdEncoding.EncodeToString([]byte("export const x = 1;\n")),
				Options:    Options{Languages: []string{"go"}},
			},
			kind: KindUnsupportedLanguage,
		},
		{
			name: "binary content",
			req:  analyzeRequest([]byte("package main\x00\n")),
			kind: KindBinaryContent,
		},
		{
			name: "file too large",
			req: func() Request {
				r := analyzeRequest([]byte("package main\n"))
				r.Options.MaxFileBytes = 4
				return r
			}(),
			kind: KindFileTooLarge,
		},
		{
			name: "invalid options",
			req: func() Request {
				r := analyzeRequest([]byte("package main\n"))
				r.Options.MaxFileBytes = -1
				return r
			}(),
			kind: KindInvalidOptions,
		},
		{
			name: "unsupported language in options",
			req: func() Request {
				r := analyzeRequest([]byte("package main\n"))
				r.Options.Languages = []string{"cobol"}
				return r
			}(),
			kind: KindUnsupportedLanguage,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			body_handlePart2Test_79(t, tc)
		})
	}
}

func loadManifest(t *testing.T) []parityCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "parity", "manifest.json"))
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var cases []parityCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parse manifest: %v", err)
	}
	return cases
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

func TestHandleUnknownOp(t *testing.T) {
	resp := Handle(context.Background(), Request{ID: 1, Op: "explode"})
	if resp.Error == nil || resp.Error.Kind != KindInternal {
		t.Fatalf("error = %+v, want kind %q", resp.Error, KindInternal)
	}
}
