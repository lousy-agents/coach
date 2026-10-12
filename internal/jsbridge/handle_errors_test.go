package jsbridge

import (
	"context"
	"encoding/base64"
	"testing"
)

func TestErrorKinds(t *testing.T) {
	tests := []errorKindCase{
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
			expectErrorKind(t, tc)
		})
	}
}

type errorKindCase struct {
	name string
	req  Request
	kind string
}

func expectErrorKind(t *testing.T, tc errorKindCase) {
	resp := Handle(context.Background(), tc.req)
	if resp.Error == nil {
		t.Fatalf("no error, want kind %q", tc.kind)
	}
	if resp.Error.Kind != tc.kind {
		t.Fatalf("kind = %q (%s), want %q", resp.Error.Kind, resp.Error.Message, tc.kind)
	}
	if resp.Result != nil {
		t.Fatalf("Result = %+v, want nil for non-syntax errors", resp.Result)
	}
}

func TestHandleUnknownOp(t *testing.T) {
	resp := Handle(context.Background(), Request{ID: 1, Op: "explode"})
	if resp.Error == nil || resp.Error.Kind != KindInternal {
		t.Fatalf("error = %+v, want kind %q", resp.Error, KindInternal)
	}
}

// TestHandleNonUTF8Content proves base64 delivers exact bytes: invalid UTF-8
// without a NUL byte must reach the parser rather than fail in transport or
// trip the binary-content check.
func TestHandleNonUTF8Content(t *testing.T) {
	content := append([]byte("package main\n// comment \xff\xfe\n"), []byte("func main() {}\n")...)
	resp := Handle(context.Background(), analyzeRequest(content))
	if resp.Error != nil && resp.Error.Kind == KindInternal {
		t.Fatalf("transport failed on non-UTF-8 bytes: %+v", resp.Error)
	}
	if resp.Error != nil && resp.Error.Kind == KindBinaryContent {
		t.Fatalf("non-NUL bytes misclassified as binary: %+v", resp.Error)
	}
}

func TestHandleBadBase64(t *testing.T) {
	resp := Handle(context.Background(), Request{ID: 1, Op: OpAnalyze, Language: "go", ContentB64: "!!!not base64!!!"})
	if resp.Error == nil || resp.Error.Kind != KindInternal {
		t.Fatalf("error = %+v, want kind %q", resp.Error, KindInternal)
	}
}

func TestHandleCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resp := Handle(ctx, analyzeRequest([]byte("package main\n")))
	if resp.Error == nil || resp.Error.Kind != KindCanceled {
		t.Fatalf("error = %+v, want kind %q", resp.Error, KindCanceled)
	}
}
