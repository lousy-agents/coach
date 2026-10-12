package tssidecar

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

func decodeResponse(data []byte, reqID int64) (projectbridge.Response, string) {
	var resp projectbridge.Response
	if err := json.Unmarshal(bytes.TrimRight(firstLine(data), "\n"), &resp); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar produced malformed response: %s", err)
	}
	if resp.Version != projectbridge.ProtocolVersion || resp.ID != reqID {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar response protocol mismatch: version %d id %d", resp.Version, resp.ID)
	}
	return resp, ""
}

func firstLine(data []byte) []byte {
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[:idx]
	}
	return data
}
