package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// layerViolationMarker stands in for a real TypeScript import statement, so
// this fixture can drive the sidecar's output without parsing TypeScript.
const layerViolationMarker = "LAYER_VIOLATION_MARKER"

// layerViolationCrashMarker is checked first, ahead of
// layerViolationHangMarker and layerViolationMarker, so a fixture can safely
// carry more than one marker.
const layerViolationCrashMarker = "LAYER_VIOLATION_CRASH_MARKER"

// layerViolationHangMarker relies on the real caller's context deadline
// (internal/codesignalcli's tsSidecarWallTime) to kill the subprocess.
// Checked ahead of layerViolationMarker for the same reason as
// layerViolationCrashMarker.
const layerViolationHangMarker = "LAYER_VIOLATION_HANG_MARKER"

func main() {
	req := readRequest()

	if hasMarkerString(req.Files, layerViolationCrashMarker) {
		fmt.Fprintln(os.Stderr, "fake_ts_sidecar: simulated crash")
		os.Exit(1)
	}

	if hasMarkerString(req.Files, layerViolationHangMarker) {
		// A bare select{} is flagged by the Go runtime as a deadlock and
		// exits immediately, exercising the crash path instead of this one.
		// The sleep keeps a runnable timer alive so the process blocks until
		// the caller's context deadline kills it.
		time.Sleep(time.Hour)
	}

	resp := projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		Coverage: projectbridge.Coverage{
			Phase:    "fake_ts_project_sidecar",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	}
	if hasMarker(req.Files) {
		resp.ImportEdges = []projectbridge.ImportEdgeFact{
			{
				From:       "file:pkg/handlers/h.ts",
				To:         "file:pkg/db/d.ts",
				Kind:       "import",
				Resolution: "snapshot",
				Site:       "pkg/handlers/h.ts:10",
			},
		}
	}

	writeResponse(resp)
}

func readRequest() projectbridge.Request {
	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, _ := reader.ReadString('\n')
	var req projectbridge.Request
	_ = json.Unmarshal([]byte(line), &req)
	return req
}

func hasMarker(files []projectbridge.ProjectFile) bool {
	return hasMarkerString(files, layerViolationMarker)
}

func hasMarkerString(files []projectbridge.ProjectFile, marker string) bool {
	for _, f := range files {
		decoded, err := base64.StdEncoding.DecodeString(f.ContentB64)
		if err != nil {
			continue
		}
		if strings.Contains(string(decoded), marker) {
			return true
		}
	}
	return false
}

func writeResponse(resp projectbridge.Response) {
	out, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
