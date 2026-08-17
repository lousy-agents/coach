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
}

// reachabilityFixtureFact is the one resolved call-graph edge/reachability
// fact modeReachability and modeReachabilityGap both emit, mirroring the
// real sidecar's own reachability-registry vocabulary (js/semantics/src/
// project-sidecar/reachability-registry.ts's REACHABILITY_ALGORITHM/
// REACHABILITY_BACKEND) so the fake stand-in exercises the same wire shape.
func reachabilityFixtureFact() projectbridge.ReachabilityFactWire {
	const source = "file:src/app.ts#getUsers"
	const sink = "(PrismaClient).findMany"
	return projectbridge.ReachabilityFactWire{
		ID:         fmt.Sprintf("reach:%s->%s@ts-source-sink-registry@1", source, sink),
		Kind:       projectbridge.KindPossibleCallReachability,
		Confidence: "resolved_direct",
		Source:     source,
		Sink:       sink,
		Path: []projectbridge.ReachabilityStepFact{
			{NodeID: source},
			{NodeID: sink},
		},
		AlgorithmVersion: "ts-source-sink-registry@1",
		Backend:          "ts_project_sidecar",
	}
}

func modeReachability(req projectbridge.Request) {
	fact := reachabilityFixtureFact()
	writeResponse(projectbridge.Response{
		Version:           req.Version,
		ID:                req.ID,
		CallGraph:         []projectbridge.CallGraphEdgeFact{{From: fact.Source, To: fact.Sink}},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{fact},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

func modeReachabilityGap(req projectbridge.Request) {
	fact := reachabilityFixtureFact()
	writeResponse(projectbridge.Response{
		Version:           req.Version,
		ID:                req.ID,
		CallGraph:         []projectbridge.CallGraphEdgeFact{{From: fact.Source, To: fact.Sink}},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{fact},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: false,
			Counts:   map[string]int{"files_seen": len(req.Files)},
			Diagnostics: []projectbridge.Diagnostic{
				{
					Code:    "ts_reachability_type_only_gap",
					Message: "call target resolves through a type-only import binding, so further reachability from here is unverified",
					Path:    "src/app.ts",
				},
			},
		},
	})
}

// modeReachabilityMulti emits three facts: two sharing
// "file:src/app.ts#getUsers" as Source (so a caller must dedup), plus a
// third fact from "file:src/app.ts#createUser" -- which sorts before
// "getUsers" -- emitted last (so a caller must sort rather than preserve
// wire order).
func modeReachabilityMulti(req projectbridge.Request) {
	fact1 := reachabilityFixtureFact()
	fact2 := projectbridge.ReachabilityFactWire{
		ID:         fmt.Sprintf("reach:%s->%s@ts-source-sink-registry@1", fact1.Source, "(PrismaClient).update"),
		Kind:       projectbridge.KindPossibleCallReachability,
		Confidence: "resolved_direct",
		Source:     fact1.Source,
		Sink:       "(PrismaClient).update",
		Path: []projectbridge.ReachabilityStepFact{
			{NodeID: fact1.Source},
			{NodeID: "(PrismaClient).update"},
		},
		AlgorithmVersion: "ts-source-sink-registry@1",
		Backend:          "ts_project_sidecar",
	}
	const source3 = "file:src/app.ts#createUser"
	const sink3 = "(PrismaClient).create"
	fact3 := projectbridge.ReachabilityFactWire{
		ID:         fmt.Sprintf("reach:%s->%s@ts-source-sink-registry@1", source3, sink3),
		Kind:       projectbridge.KindPossibleCallReachability,
		Confidence: "resolved_direct",
		Source:     source3,
		Sink:       sink3,
		Path: []projectbridge.ReachabilityStepFact{
			{NodeID: source3},
			{NodeID: sink3},
		},
		AlgorithmVersion: "ts-source-sink-registry@1",
		Backend:          "ts_project_sidecar",
	}
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		CallGraph: []projectbridge.CallGraphEdgeFact{
			{From: fact1.Source, To: fact1.Sink},
			{From: fact2.Source, To: fact2.Sink},
			{From: fact3.Source, To: fact3.Sink},
		},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{fact1, fact2, fact3},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

// layerBypassFact returns a resolved reachability fact for (source, sink),
// mirroring reachabilityFixtureFact's shape but parameterized so the
// layer-bypass modes below can register whichever Source/Sink pair their
// CallGraph fixture actually resolves to.
func layerBypassFact(source, sink string) projectbridge.ReachabilityFactWire {
	return projectbridge.ReachabilityFactWire{
		ID:               fmt.Sprintf("reach:%s->%s@ts-source-sink-registry@1", source, sink),
		Kind:             projectbridge.KindPossibleCallReachability,
		Confidence:       "resolved_direct",
		Source:           source,
		Sink:             sink,
		Path:             []projectbridge.ReachabilityStepFact{{NodeID: source}, {NodeID: sink}},
		AlgorithmVersion: "ts-source-sink-registry@1",
		Backend:          "ts_project_sidecar",
	}
}

// modeLayerBypassDirect emits exactly the real sidecar's own depth-1 call-
// graph edge shape (js/semantics/src/project-sidecar/reachability.ts): a
// single route-handler-to-sink edge, with no same-layer call-graph node.
// The required-layer match comes from the acceptance suite's own snapshot
// containing a real file under "service/", not from this CallGraph.
func modeLayerBypassDirect(req projectbridge.Request) {
	const source = "file:src/handlers/app.ts#getUsers"
	const sink = "(PrismaClient).findMany"
	writeResponse(projectbridge.Response{
		Version:           req.Version,
		ID:                req.ID,
		CallGraph:         []projectbridge.CallGraphEdgeFact{{From: source, To: sink}},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{layerBypassFact(source, sink)},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

// modeLayerBypassCompliant emits a source node whose own declaration
// directory ("service") falls under the "service" required-layer prefix the
// acceptance test configures -- BuildTypeScriptLayerBypass must remove this
// node from adjacency entirely rather than emit a witness for it.
func modeLayerBypassCompliant(req projectbridge.Request) {
	const source = "file:service/handler.ts#getUsers"
	const sink = "(PrismaClient).findMany"
	writeResponse(projectbridge.Response{
		Version:           req.Version,
		ID:                req.ID,
		CallGraph:         []projectbridge.CallGraphEdgeFact{{From: source, To: sink}},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{layerBypassFact(source, sink)},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

// modeLayerBypassDual emits a synthetic multi-hop call graph -- a shape the
// TS sidecar's real depth-1 walk does not itself produce today -- so the
// acceptance suite can exercise BuildTypeScriptLayerBypass's general
// BFS/shortest-witness tie-break over more than one hop: two equal-length
// routes from Handler to the sink (via AlphaQuery and BetaQuery), plus an
// unrelated required-layer edge that must be removed from adjacency before
// the search runs.
func modeLayerBypassDual(req projectbridge.Request) {
	const handler = "file:src/app.ts#Handler"
	const alpha = "file:src/app.ts#AlphaQuery"
	const beta = "file:src/app.ts#BetaQuery"
	const unused = "file:service/unused.ts#Unused"
	const sink = "(PrismaClient).findMany"
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		CallGraph: []projectbridge.CallGraphEdgeFact{
			{From: handler, To: alpha},
			{From: handler, To: beta},
			{From: handler, To: unused},
			{From: alpha, To: sink},
			{From: beta, To: sink},
		},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{layerBypassFact(handler, sink)},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

// modeLayerBypassCycle emits a synthetic call graph containing a cycle
// (CycleA <-> CycleB) unrelated to the shortest path to the sink, proving
// BuildTypeScriptLayerBypass's BFS terminates and finds the correct witness
// deterministically rather than hanging or double-visiting nodes. The
// "service" required layer resolves from the acceptance suite's own
// snapshot file inventory, not from any node in this CallGraph.
func modeLayerBypassCycle(req projectbridge.Request) {
	const handler = "file:src/app.ts#Handler"
	const cycleA = "file:src/app.ts#CycleA"
	const cycleB = "file:src/app.ts#CycleB"
	const queryDB = "file:src/app.ts#QueryDB"
	const sink = "(PrismaClient).findMany"
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		CallGraph: []projectbridge.CallGraphEdgeFact{
			{From: handler, To: cycleA},
			{From: cycleA, To: cycleB},
			{From: cycleB, To: cycleA},
			{From: handler, To: queryDB},
			{From: queryDB, To: sink},
		},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{layerBypassFact(handler, sink)},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: true,
			Counts:   map[string]int{"files_seen": len(req.Files)},
		},
	})
}

// modeLayerBypassGap emits a resolvable bypass edge alongside
// Coverage.Complete false and an unrelated gap diagnostic --
// BuildTypeScriptLayerBypass must still emit this pair's own resolved
// witness while reporting the aggregate LayerBypassResult.Coverage.Complete
// as false. The "service" required layer resolves from the acceptance
// suite's own snapshot file inventory (tsLayerBypassSnapshot's
// "service/inventory.ts"), independent of this CallGraph.
func modeLayerBypassGap(req projectbridge.Request) {
	const source = "file:src/handlers/app.ts#getUsers"
	const sink = "(PrismaClient).findMany"
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		CallGraph: []projectbridge.CallGraphEdgeFact{
			{From: source, To: sink},
		},
		ReachabilityFacts: []projectbridge.ReachabilityFactWire{layerBypassFact(source, sink)},
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: false,
			Counts:   map[string]int{"files_seen": len(req.Files)},
			Diagnostics: []projectbridge.Diagnostic{
				{
					Code:    "ts_reachability_type_only_gap",
					Message: "call target resolves through a type-only import binding, so further reachability from here is unverified",
					Path:    "src/app.ts",
				},
			},
		},
	})
}

func modeBadTsconfig(req projectbridge.Request) {
	writeResponse(projectbridge.Response{
		Version: req.Version,
		ID:      req.ID,
		Coverage: projectbridge.Coverage{
			Phase:    "ts_sidecar_fake",
			Complete: false,
			Counts:   map[string]int{"files_seen": len(req.Files)},
			Diagnostics: []projectbridge.Diagnostic{
				{Code: "ts_config_diagnostic", Message: "simulated unparseable tsconfig.json", Path: "tsconfig.json"},
			},
		},
	})
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
