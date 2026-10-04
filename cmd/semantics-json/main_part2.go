package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"

	"github.com/lousy-agents/coach/internal/jsbridge"

	"os"
)

// handleLine decodes one request line and runs it through the bridge. A
// line that isn't valid Request JSON gets id 0 — unattributable, which the
// JS side treats as fatal for its child process.
func handleLine(ctx context.Context, line []byte) jsbridge.Response {
	var req jsbridge.Request
	if err := json.Unmarshal(line, &req); err != nil {
		return jsbridge.Response{
			Error: &jsbridge.ErrorPayload{
				Kind:    jsbridge.KindInternal,
				Message: fmt.Sprintf("semantics-json: malformed request line: %v", err),
			},
		}
	}
	return jsbridge.Handle(ctx, req)
}
func main() {
	once := flag.Bool("once", false, "read exactly one request, respond, and exit")
	flag.Parse()

	if err := serve(context.Background(), os.Stdin, os.Stdout, *once); err != nil {
		fmt.Fprintf(os.Stderr, "semantics-json: %v\n", err)
		os.Exit(1)
	}
}
