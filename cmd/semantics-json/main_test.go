package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/jsbridge"
)

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

func TestServeSkipsBlankLines(t *testing.T) {
	input := "\n  \n" + request(t, 5, "package main\n") + "\n\n"
	responses := serveLines(t, input, false)
	if len(responses) != 1 || responses[0].ID != 5 {
		t.Fatalf("responses = %+v, want exactly one with ID 5", responses)
	}
}

func TestServeMalformedLineGetsIDZero(t *testing.T) {
	input := "this is not json\n" + request(t, 9, "package main\n") + "\n"
	responses := serveLines(t, input, false)
	if len(responses) != 2 {
		t.Fatalf("got %d responses, want 2", len(responses))
	}
	if responses[0].ID != 0 || responses[0].Error == nil || responses[0].Error.Kind != jsbridge.KindInternal {
		t.Errorf("malformed line response = %+v, want ID 0 with kind internal", responses[0])
	}
	if responses[1].ID != 9 {
		t.Errorf("server did not keep serving after a malformed line: %+v", responses[1])
	}
}

func TestServeOnceStopsAfterOneRequest(t *testing.T) {
	input := request(t, 1, "package main\n") + "\n" + request(t, 2, "package main\n") + "\n"
	responses := serveLines(t, input, true)
	if len(responses) != 1 || responses[0].ID != 1 {
		t.Fatalf("responses = %+v, want exactly the first", responses)
	}
}

func TestServeOnceWithoutInputFails(t *testing.T) {
	var out strings.Builder
	if err := serve(context.Background(), strings.NewReader(""), &out, true); err == nil {
		t.Fatal("serve --once on empty input succeeded, want error")
	}
}

// TestServeLargeContent pushes a request line well past bufio.Scanner's
// 64 KB default to prove the enlarged buffer holds.
func TestServeLargeContent(t *testing.T) {
	var source strings.Builder
	source.WriteString("package main\n")
	for i := 0; i < 20000; i++ {
		fmt.Fprintf(&source, "// filler comment line %d to inflate the file\n", i)
	}
	responses := serveLines(t, request(t, 42, source.String())+"\n", false)
	if len(responses) != 1 {
		t.Fatalf("got %d responses, want 1", len(responses))
	}
	if responses[0].Error != nil {
		t.Fatalf("unexpected error: %+v", responses[0].Error)
	}
	if responses[0].Result == nil || responses[0].Result.ParseStatus != "ok" {
		t.Fatalf("result = %+v, want parse_status ok", responses[0].Result)
	}
}
