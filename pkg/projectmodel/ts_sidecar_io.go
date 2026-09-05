package projectmodel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

func decodeTSSidecarResponse(data []byte, reqID int64) (projectbridge.Response, string) {
	var resp projectbridge.Response
	if err := json.Unmarshal(bytes.TrimRight(firstLine(data), "\n"), &resp); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar produced malformed response: %s", err)
	}
	if resp.Version != projectbridge.ProtocolVersion || resp.ID != reqID {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar response protocol mismatch: version %d id %d", resp.Version, resp.ID)
	}
	return resp, ""
}

// sanitizedTSSidecarEnv is the analyzer child's environment. Only PATH is
// forwarded: Node is spawned by absolute path, but some native compiler
// packages still consult PATH for the dynamic linker. HOME, NODE_OPTIONS,
// HTTP(S)_PROXY, npm_config_*, and every other ambient variable are
// omitted so a leaked loader, proxy, or package-manager config cannot
// influence analysis (AC-RUN-2/AC-RUN-4).
func sanitizedTSSidecarEnv() []string {
	var env []string
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	return env
}

// boundedWriter retains only the first limit bytes written to it, silently
// discarding the rest, so an untrusted child's stderr cannot grow the
// diagnostic message without bound.
type boundedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}

func firstLine(data []byte) []byte {
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[:idx]
	}
	return data
}
