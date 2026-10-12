package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

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

func TestServeSkipsBlankLines(t *testing.T) {
	input := "\n  \n" + request(t, 5, "package main\n") + "\n\n"
	responses := serveLines(t, input, false)
	if len(responses) != 1 || responses[0].ID != 5 {
		t.Fatalf("responses = %+v, want exactly one with ID 5", responses)
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
