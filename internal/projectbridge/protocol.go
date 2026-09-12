// Package projectbridge defines the JSON protocol between
// pkg/projectmodel's TypeScript sidecar client and a pinned local
// Node/TypeScript sidecar subprocess that produces raw TypeScript/TSX
// import facts from an immutable snapshot (issue #214).
//
// It deliberately mirrors internal/jsbridge's conventions -- an Op
// selector, a correlation ID, a typed ErrorPayload.Kind carried across the
// process boundary in place of errors.Is, and a bounded read loop -- and
// adds an explicit Version field that jsbridge's Request/Response do not
// have. The transport direction is reversed: jsbridge exposes a Go
// analyzer to a JS/WASM caller, so its Go side is the server. Here Go is
// the client: only the real TypeScript compiler (which runs only in Node)
// can resolve tsconfig project references, path aliases, and
// package.json exports/re-exports, so pkg/projectmodel spawns the
// sidecar, writes one Request to its stdin, and reads one Response back
// from its stdout.
//
// The package lives under internal/ deliberately: the wire format is an
// implementation detail of this specific boundary, not public Go API.
package projectbridge

const ProtocolVersion = 1

const OpAnalyzeProject = "analyze_project"

type ProjectFile struct {
	Path       string `json:"path"`
	ContentB64 string `json:"content_b64"`
}

type Request struct {
	Version   int           `json:"version"`
	Op        string        `json:"op"`
	ID        int64         `json:"id"`
	Files     []ProjectFile `json:"files"`
	Roots     []string      `json:"roots,omitempty"`
	TimeoutMS int64         `json:"timeout_ms,omitempty"`
}

type ImportEdgeFact struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Site       string `json:"site,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

type CallGraphEdgeFact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type RootScopeFact struct {
	Root            string   `json:"root"`
	CandidateFiles  int      `json:"candidate_files"`
	AnalyzedFiles   int      `json:"analyzed_files"`
	AnalyzedPaths   []string `json:"analyzed_paths,omitempty"`
	UnanalyzedPaths []string `json:"unanalyzed_paths,omitempty"`
}

// KindPossibleCallReachability mirrors projectmodel.KindPossibleCallReachability.
// Defined separately here (not imported) for the same import-cycle reason
// documented on Diagnostic.
const KindPossibleCallReachability = "possible_call_reachability"

type ReachabilityStepFact struct {
	NodeID string `json:"node_id"`
}

type ReachabilityFactWire struct {
	ID               string                 `json:"id"`
	Kind             string                 `json:"kind"`
	Confidence       string                 `json:"confidence"`
	Source           string                 `json:"source"`
	Sink             string                 `json:"sink"`
	Path             []ReachabilityStepFact `json:"path"`
	AlgorithmVersion string                 `json:"algorithm_version"`
	Backend          string                 `json:"backend,omitempty"`
}

// Diagnostic mirrors projectmodel.Diagnostic's exact field names and JSON
// shape. It is a separate Go type, not a re-export, because
// pkg/projectmodel imports this package (the client imports its own wire
// format) and importing projectmodel back here would create a cycle; the
// shape is the contract that must never drift, not the identity of the
// Go type.
type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
}

// Coverage mirrors projectmodel.Coverage's exact field names and JSON
// shape for the same import-cycle reason documented on Diagnostic.
type Coverage struct {
	Phase       string         `json:"phase"`
	Complete    bool           `json:"complete"`
	Counts      map[string]int `json:"counts,omitempty"`
	Budgets     map[string]int `json:"budgets,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

type Response struct {
	Version           int                    `json:"version"`
	ID                int64                  `json:"id"`
	ImportEdges       []ImportEdgeFact       `json:"import_edges,omitempty"`
	CallGraph         []CallGraphEdgeFact    `json:"call_graph,omitempty"`
	ReachabilityFacts []ReachabilityFactWire `json:"reachability_facts,omitempty"`
	RootScopes        []RootScopeFact        `json:"root_scopes,omitempty"`
	Coverage          Coverage               `json:"coverage"`
	Error             *ErrorPayload          `json:"error,omitempty"`
}

// ErrorPayload is the wire form of a whole-request sidecar error, mirroring
// internal/jsbridge.ErrorPayload.
type ErrorPayload struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

const (
	KindBackendUnavailable = "backend_unavailable"
	KindCrashed            = "crashed"
	KindMalformedOutput    = "internal"
	KindCanceled           = "canceled"
)
