// Command fake_ts_sidecar is a test-only stand-in for the real Node/
// TypeScript sidecar (issue #214 Task 2). It speaks the same
// internal/projectbridge NDJSON protocol over stdin/stdout and can be
// told, via a --mode flag, to simulate each failure mode the real
// sidecar could someday produce: a transport-level crash, a crash that
// also floods stderr well past the client's retained-diagnostic budget
// (crash_noisy), malformed output, oversized output, a hang, a protocol
// version mismatch or correlation-id mismatch (each in isolation, so the
// client's check of each half of the mismatch condition is independently
// covered), a whole-request Response.Error (request_error), an
// env-sanitization probe (env, reporting what the child process actually
// received), an outgoing-request probe (request_probe, echoing the
// received Op/TimeoutMS/Roots back in a diagnostic), a valid response
// line followed by stray non-JSON trailing stdout (trailing_output,
// simulating a transitively-loaded module's stray console.log), or a
// successful-but-incomplete response (partial, Complete false with
// Budgets set and no Error). It is compiled on demand by
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

// oversizedFillerBytes exceeds the client's response-size budget so the
// "oversized" mode reliably trips it regardless of the exact budget value.
const oversizedFillerBytes = 16 << 20 // 16 MiB

// noisyStderrFillerBytes exceeds the client's maxTSSidecarStderrBytes
// retention budget (4 KiB) so the "crash_noisy" mode reliably proves the
// client caps captured stderr regardless of the exact budget value.
const noisyStderrFillerBytes = 64 << 10 // 64 KiB

func main() {
	mode := "happy"
	for _, arg := range os.Args[1:] {
		if rest, ok := strings.CutPrefix(arg, "--mode="); ok {
			mode = rest
		}
	}

	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, _ := reader.ReadString('\n')

	var req projectbridge.Request
	_ = json.Unmarshal([]byte(line), &req)

	switch mode {
	case "crash":
		fmt.Fprintln(os.Stderr, "fake_ts_sidecar: simulated crash")
		os.Exit(3)
	case "crash_noisy":
		fmt.Fprintln(os.Stderr, "fake_ts_sidecar: simulated noisy crash")
		fmt.Fprint(os.Stderr, strings.Repeat("x", noisyStderrFillerBytes))
		os.Exit(3)
	case "malformed":
		fmt.Println("{not valid json")
	case "oversized":
		fmt.Println(strings.Repeat("x", oversizedFillerBytes))
	case "hang":
		time.Sleep(30 * time.Second)
	case "version_mismatch":
		resp := projectbridge.Response{
			Version: projectbridge.ProtocolVersion + 1,
			ID:      req.ID,
			Coverage: projectbridge.Coverage{
				Phase:    "ts_sidecar_fake",
				Complete: true,
			},
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "id_mismatch":
		resp := projectbridge.Response{
			Version: req.Version,
			ID:      req.ID + 1,
			Coverage: projectbridge.Coverage{
				Phase:    "ts_sidecar_fake",
				Complete: true,
			},
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "request_error":
		resp := projectbridge.Response{
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
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "request_probe":
		resp := projectbridge.Response{
			Version: req.Version,
			ID:      req.ID,
			Coverage: projectbridge.Coverage{
				Phase:    "ts_sidecar_fake",
				Complete: true,
				Diagnostics: []projectbridge.Diagnostic{
					{Code: "request_probe", Message: fmt.Sprintf("op=%q timeout_ms=%d roots=%v", req.Op, req.TimeoutMS, req.Roots)},
				},
			},
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "partial":
		resp := projectbridge.Response{
			Version: req.Version,
			ID:      req.ID,
			Coverage: projectbridge.Coverage{
				Phase:    "ts_sidecar_fake",
				Complete: false,
				Counts:   map[string]int{"files_seen": len(req.Files)},
				Budgets:  map[string]int{"wall_time_ms": 1234},
			},
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	case "trailing_output":
		resp := projectbridge.Response{
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
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
		fmt.Println("stray console.log output from a transitively-loaded module")
		fmt.Println(`{"not": "part of the response"}`)
	case "env":
		probe := os.Getenv("COACH_TS_SIDECAR_ENV_PROBE")
		pathState := "unset"
		if _, ok := os.LookupEnv("PATH"); ok {
			pathState = "set"
		}
		homeState := "unset"
		if _, ok := os.LookupEnv("HOME"); ok {
			homeState = "set"
		}
		resp := projectbridge.Response{
			Version: req.Version,
			ID:      req.ID,
			Coverage: projectbridge.Coverage{
				Phase:    "ts_sidecar_fake",
				Complete: true,
				Diagnostics: []projectbridge.Diagnostic{
					{Code: "env_probe", Message: fmt.Sprintf("probe=%q path=%s home=%s", probe, pathState, homeState)},
				},
			},
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	default:
		// Echoes the decoded content of the first request file back as the
		// edge's To value so a test can pin the base64 round-trip, not just
		// the count of files sent.
		to := "file:src/b.ts"
		if len(req.Files) > 0 {
			if decoded, err := base64.StdEncoding.DecodeString(req.Files[0].ContentB64); err == nil {
				to = string(decoded)
			}
		}
		resp := projectbridge.Response{
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
		}
		out, err := json.Marshal(resp)
		if err != nil {
			fmt.Fprintln(os.Stderr, "fake_ts_sidecar: marshaling response:", err)
			os.Exit(1)
		}
		fmt.Println(string(out))
	}
}
