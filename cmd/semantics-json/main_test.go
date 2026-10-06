package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/jsbridge"
)

func serveLines(t *testing.T, input string, once bool) []jsbridge.Response {
	t.Helper()
	var out strings.Builder
	if err := serve(context.Background(), strings.NewReader(input), &out, once); err != nil {
		t.Fatalf("serve: %v", err)
	}
	var responses []jsbridge.Response
	for _, line := range strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n") {
		if line == "" {
			continue
		}
		var resp jsbridge.Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("response line is not valid JSON: %v\nline: %s", err, line)
		}
		responses = append(responses, resp)
	}
	return responses
}

func request(t *testing.T, id int64, content string) string {
	t.Helper()
	encoded, err := json.Marshal(jsbridge.Request{
		ID:         id,
		Op:         jsbridge.OpAnalyze,
		Path:       "main.go",
		Language:   "go",
		ContentB64: base64.StdEncoding.EncodeToString([]byte(content)),
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return string(encoded)
}

func TestServeSequentialRequests(t *testing.T) {
	input := request(t, 1, "package main\n") + "\n" +
		request(t, 2, "package main\nfunc oops( {\n") + "\n" +
		request(t, 3, "package other\n") + "\n"
	responses := serveLines(t, input, false)
	if len(responses) != 3 {
		t.Fatalf("got %d responses, want 3", len(responses))
	}
	for i, wantID := range []int64{1, 2, 3} {
		if responses[i].ID != wantID {
			t.Errorf("response[%d].ID = %d, want %d (in-order responses)", i, responses[i].ID, wantID)
		}
	}
	if responses[0].Error != nil {
		t.Errorf("response 1: unexpected error %+v", responses[0].Error)
	}
	if responses[1].Error == nil || responses[1].Error.Kind != jsbridge.KindSyntax {
		t.Errorf("response 2: error = %+v, want kind syntax", responses[1].Error)
	}
	if responses[1].Result == nil {
		t.Error("response 2: missing partial result alongside syntax error")
	}
}
