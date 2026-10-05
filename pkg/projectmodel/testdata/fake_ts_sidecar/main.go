// Command fake_ts_sidecar is a test-only stand-in for the real Node/
// TypeScript sidecar (issue #214 Task 2). It speaks the same
// internal/projectbridge NDJSON protocol over stdin/stdout and can be
// told, via a --mode flag, to simulate each failure mode the real
// sidecar could someday produce. It is compiled on demand by
// ts_sidecar_acceptance_test.go and is not part of any production build.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

const oversizedFillerBytes = 16 << 20   // 16 MiB
const noisyStderrFillerBytes = 64 << 10 // 64 KiB

type modeHandler func(req projectbridge.Request)

func main() {
	mode := parseMode(os.Args[1:])
	recordInvocation(parseFlag(os.Args[1:], "--invocation-counter-file="))
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

// parseFlag returns the value following prefix in the first matching arg, or
// "" if prefix appears nowhere.
func parseFlag(args []string, prefix string) string {
	for _, arg := range args {
		if rest, ok := strings.CutPrefix(arg, prefix); ok {
			return rest
		}
	}
	return ""
}

// recordInvocation appends one line to counterFile per process invocation,
// letting an acceptance test count how many separate sidecar subprocess
// round trips a call sequence actually made. A no-op when counterFile is ""
// (the flag unset), so every existing mode/spec that never sets
// --invocation-counter-file is unaffected.
func recordInvocation(counterFile string) {
	if counterFile == "" {
		return
	}
	f, err := os.OpenFile(counterFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake_ts_sidecar: recording invocation:", err)
		return
	}
	defer f.Close()
	fmt.Fprintln(f, "1")
}

func readRequest() projectbridge.Request {
	reader := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, _ := reader.ReadString('\n')
	var req projectbridge.Request
	_ = json.Unmarshal([]byte(line), &req)
	return req
}

var modeHandlers = map[string]modeHandler{
	"crash":                  modeCrash,
	"crash_noisy":            modeCrashNoisy,
	"malformed":              modeMalformed,
	"oversized":              modeOversized,
	"hang":                   modeHang,
	"version_mismatch":       modeVersionMismatch,
	"id_mismatch":            modeIDMismatch,
	"request_error":          modeRequestError,
	"request_probe":          modeRequestProbe,
	"partial":                modePartial,
	"trailing_output":        modeTrailingOutput,
	"env":                    modeEnv,
	"cwd":                    modeCwd,
	"happy":                  modeHappy,
	"reachability":           modeReachability,
	"reachability_gap":       modeReachabilityGap,
	"reachability_multi":     modeReachabilityMulti,
	"bad_tsconfig":           modeBadTsconfig,
	"layer_bypass_direct":    modeLayerBypassDirect,
	"layer_bypass_compliant": modeLayerBypassCompliant,
	"layer_bypass_dual":      modeLayerBypassDual,
	"layer_bypass_cycle":     modeLayerBypassCycle,
	"layer_bypass_gap":       modeLayerBypassGap,
	"unanalyzable_candidate": modeUnanalyzableCandidate,
	"root_scope_missing":     modeRootScopeMissing,
}

// reachabilityFixtureFact is the one resolved call-graph edge/reachability
// fact modeReachability and modeReachabilityGap both emit, mirroring the
// real sidecar's own reachability-registry vocabulary (js/semantics/src/
// project-sidecar/reachability-registry.ts's REACHABILITY_ALGORITHM/
// REACHABILITY_BACKEND) so the fake stand-in exercises the same wire shape.
