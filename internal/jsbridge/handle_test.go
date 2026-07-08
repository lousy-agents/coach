package jsbridge

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
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

// TestParityFixtures locks the exact Response JSON for every manifest case.
// The expected files are authoritative-by-construction: `go test -run
// TestParityFixtures ./internal/jsbridge -update` regenerates them from
// whatever Handle actually emits, and this test fails on any drift, so the
// JS parity suite always compares against Go-canonical bytes.
func TestParityFixtures(t *testing.T) {
	for _, tc := range loadManifest(t) {
		t.Run(tc.Name, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", "parity", tc.Src))
			if err != nil {
				t.Fatalf("read src: %v", err)
			}
			resp := Handle(context.Background(), Request{
				Op:         OpAnalyze,
				Path:       tc.Path,
				Language:   tc.Language,
				ContentB64: base64.StdEncoding.EncodeToString(content),
				Options:    tc.Options,
			})
			got, err := json.MarshalIndent(resp, "", "  ")
			if err != nil {
				t.Fatalf("marshal response: %v", err)
			}
			got = append(got, '\n')

			expectedPath := filepath.Join("testdata", "parity", tc.Expected)
			if *update {
				if err := os.WriteFile(expectedPath, got, 0o644); err != nil {
					t.Fatalf("write expected: %v", err)
				}
				return
			}
			want, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("read expected (run with -update to generate): %v", err)
			}
			if string(got) != string(want) {
				t.Errorf("response drifted from %s\ngot:\n%s\nwant:\n%s", tc.Expected, got, want)
			}
		})
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

func TestHandleEchoesID(t *testing.T) {
	resp := Handle(context.Background(), analyzeRequest([]byte("package main\n")))
	if resp.ID != 7 {
		t.Fatalf("ID = %d, want 7", resp.ID)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
}

func TestHandleUnknownOp(t *testing.T) {
	resp := Handle(context.Background(), Request{ID: 1, Op: "explode"})
	if resp.Error == nil || resp.Error.Kind != KindInternal {
		t.Fatalf("error = %+v, want kind %q", resp.Error, KindInternal)
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
		})
	}
}
