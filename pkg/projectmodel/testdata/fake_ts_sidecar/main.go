// Command fake_ts_sidecar is a test-only stand-in for the real Node/
// TypeScript sidecar (issue #214 Task 2). It speaks the same
// internal/projectbridge NDJSON protocol over stdin/stdout and can be
// told, via a --mode flag, to simulate each failure mode the real
// sidecar could someday produce. It is compiled on demand by
// ts_sidecar_acceptance_test.go and is not part of any production build.
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

const oversizedFillerBytes = 16 << 20   // 16 MiB
const noisyStderrFillerBytes = 64 << 10 // 64 KiB

type modeHandler func(req projectbridge.Request)

func main() {
	mode := parseMode(os.Args[1:])
	req := readRequest()
	handler, ok := modeHandlers[mode]
	if !ok {
		handler = modeHappy
	}
	handler(req)
}

func parseMode(args []string) string {
	mode := "happy"
	for _, arg := range args {
		if rest, ok := strings.CutPrefix(arg, "--mode="); ok {
			mode = rest
		}
	}
	return mode
}

func readRequest() projectbridge.Request {
	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, _ := reader.ReadString('\n')
	var req projectbridge.Request
	_ = json.Unmarshal([]byte(line), &req)
	return req
}

var modeHandlers = map[string]modeHandler{
	"crash":            modeCrash,
	"crash_noisy":      modeCrashNoisy,
	"malformed":        modeMalformed,
	"oversized":        modeOversized,
	"hang":             modeHang,
	"version_mismatch": modeVersionMismatch,
	"id_mismatch":      modeIDMismatch,
	"request_error":    modeRequestError,
	"request_probe":    modeRequestProbe,
	"partial":          modePartial,
	"trailing_output":  modeTrailingOutput,
	"env":              modeEnv,
	"happy":            modeHappy,
}

func modeCrash(_ projectbridge.Request) {
	fmt.Fprintln(os.Stderr, "fake_ts_sidecar: simulated crash")
	os.Exit(3)
}

func modeCrashNoisy(_ projectbridge.Request) {
	fmt.Fprintln(os.Stderr, "fake_ts_sidecar: simulated noisy crash")
	fmt.Fprint(os.Stderr, strings.Repeat("x", noisyStderrFillerBytes))
	os.Exit(3)
}

func modeMalformed(_ projectbridge.Request) {
	fmt.Println("{not valid json")
}

func modeOversized(_ projectbridge.Request) {
	fmt.Println(strings.Repeat("x", oversizedFillerBytes))
}

func modeHang(_ projectbridge.Request) {
	time.Sleep(30 * time.Second)
}

func modeVersionMismatch(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version:  projectbridge.ProtocolVersion + 1,
		ID:       req.ID,
		Coverage: projectbridge.Coverage{Phase: "ts_sidecar_fake", Complete: true},
	})
}

func modeIDMismatch(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version:  req.Version,
		ID:       req.ID + 1,
		Coverage: projectbridge.Coverage{Phase: "ts_sidecar_fake", Complete: true},
	})
}

func modeRequestError(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		Error: &projectbridge.ErrorPayload{
			Kind:    projectbridge.KindCrashed,
			Message: "simulated tsconfig read failure",
		},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: false,
			Diagnostics: []projectbridge.Diagnostic{
				{Code: "ts_partial_parse", Message: "partial parse before failure", Path: "src/a.ts"},
			},
		},
	})
}

func modeRequestProbe(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Diagnostics: []projectbridge.Diagnostic{
				{Code: "request_probe", Message: fmt.Sprintf("op=%q timeout_ms=%d roots=%v", req.Op, req.TimeoutMS, req.Roots)},
			},
		},
	})
}

func modePartial(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: false,
			Counts:   map[string]int{"files_seen": len(req.Files)},
			Budgets:  map[string]int{"wall_time_ms": 1234},
		},
	})
}

func modeTrailingOutput(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		ImportEdges: []projectbridge.ImportEdgeFact{
			{From: "file:src/a.ts", To: "file:src/b.ts", Kind: "internal", Site: "src/a.ts:1"},
		},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
	fmt.Println("stray console.log output from a transitively-loaded module")
	fmt.Println(`{"not": "part of the response"}`)
}

func modeEnv(req projectbridge.Request) {
	probe := os.Getenv("COACH_TS_SIDECAR_ENV_PROBE")
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Diagnostics: []projectbridge.Diagnostic{
				{Code: "env_probe", Message: fmt.Sprintf("probe=%q path=%s home=%s", probe, envState("PATH"), envState("HOME"))},
			},
		},
	})
}

func modeHappy(req projectbridge.Request) {
	to := "file:src/b.ts"
	if len(req.Files) > 0 {
		if decoded, err := base64.StdEncoding.DecodeString(req.Files[0].ContentB64); err == nil {
			to = string(decoded)
		}
	}
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		ImportEdges: []projectbridge.ImportEdgeFact{
			{From: "file:src/a.ts", To: to, Kind: "internal", Site: "src/a.ts:1"},
		},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

func envState(key string) string {
	if _, ok := os.LookupEnv(key); ok {
		return "set"
	}
	return "unset"
}

func writeResponse(resp projectbridge.Response) {
	out, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}
