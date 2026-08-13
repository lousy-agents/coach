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

// sanitizedTSSidecarEnv is the minimal environment for the sidecar child,
// mirroring internal/codesignalcli/project_snapshot.go's
// sanitizedSnapshotGitEnv: only PATH and HOME are forwarded so the
// sidecar can locate itself and any runtime it wraps (e.g. a Node
// installation), never the parent process's full ambient environment.
func sanitizedTSSidecarEnv() []string {
	var env []string
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
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
