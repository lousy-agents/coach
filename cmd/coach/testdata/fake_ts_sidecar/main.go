// Command fake_ts_sidecar is a test-only stand-in for the real TypeScript
// project sidecar (js/semantics/scripts/build-project-sidecar.mjs), used
// only by cmd/coach's project_ts_backend_acceptance_test.go. It speaks the
// internal/projectbridge NDJSON protocol over stdin/stdout: it reads one
// Request and always replies with one successful Response, emitting a
// single fixed import edge (pkg/handlers/h.ts -> pkg/db/d.ts) whenever any
// received snapshot file's decoded content contains the literal marker
// string layerViolationMarker, and zero edges otherwise. It is compiled on
// demand by the acceptance suite and is not part of any production build.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// layerViolationMarker lets a test control, via committed file content
// alone, whether this fake sidecar reports the fixed forbidden-layer edge --
// mirroring how the real sidecar's output depends on actual import
// statements, without this fixture needing to parse TypeScript.
const layerViolationMarker = "LAYER_VIOLATION_MARKER"

func main() {
	req := readRequest()

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
	for _, f := range files {
		decoded, err := base64.StdEncoding.DecodeString(f.ContentB64)
		if err != nil {
			continue
		}
		if strings.Contains(string(decoded), layerViolationMarker) {
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
