package jsbridge

import (
	"context"

	"testing"
)

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
