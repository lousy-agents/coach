package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/jsbridge"
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

func writeResponse(writer *bufio.Writer, resp jsbridge.Response) error {
	encoded, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if _, err := writer.Write(encoded); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	if err := writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("flush response: %w", err)
	}
	return nil
}
