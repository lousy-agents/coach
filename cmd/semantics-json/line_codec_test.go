package main

import (
	"testing"

	"github.com/lousy-agents/coach/internal/jsbridge"
)

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
