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

// ProtocolVersion is the wire protocol version this package implements.
const ProtocolVersion = 1

// OpAnalyzeProject is the only operation in protocol version 1.
const OpAnalyzeProject = "analyze_project"

// ProjectFile is one snapshot file made available to the sidecar.
type ProjectFile struct {
	// Path is repository-relative, matching projectmodel.File.Path.
	Path string `json:"path"`
	// ContentB64 is the exact source bytes, standard base64-encoded,
	// mirroring internal/jsbridge.Request.ContentB64 so non-UTF-8/NUL
	// content survives the JSON boundary unchanged.
	ContentB64 string `json:"content_b64"`
}

// Request is one JSON request sent to the sidecar's stdin, terminated by a
// single newline.
type Request struct {
	// Version is the sender's ProtocolVersion, echoed by the sidecar so a
	// future incompatible change fails closed instead of silently
	// misparsing.
	Version int `json:"version"`
	// Op selects the operation; only OpAnalyzeProject exists in v1.
	Op string `json:"op"`
	// ID correlates a Response with its Request; it is echoed verbatim.
	ID int64 `json:"id"`
	// Files is the snapshot's file inventory needed for TS analysis.
	Files []ProjectFile `json:"files"`
	// Roots optionally scopes analysis to specific repository-relative
	// project roots; empty means the sidecar decides.
	Roots []string `json:"roots,omitempty"`
	// TimeoutMS bounds sidecar-side analysis; 0 means none. Mirrors
	// internal/jsbridge.Request.TimeoutMS's context.WithTimeout pattern,
	// letting the sidecar self-enforce the same deadline the Go client
	// also enforces on its side of the pipe.
	TimeoutMS int64 `json:"timeout_ms,omitempty"`
}

// ImportEdgeFact is one raw import edge reported by the sidecar. Its
// fields mirror projectmodel.ImportEdge's From/To/Kind/Site/Resolution
// exactly, since the client maps a Response's ImportEdges directly into
// projectmodel.ImportEdge values without renaming any field.
type ImportEdgeFact struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Site       string `json:"site,omitempty"`
	Resolution string `json:"resolution,omitempty"`
}

// CallGraphEdgeFact is one raw call-graph edge reported by the sidecar,
// mirroring projectmodel.CallFact's From/To fields exactly.
type CallGraphEdgeFact struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// KindPossibleCallReachability is ReachabilityFactWire.Kind's fixed value,
// mirroring projectmodel.KindPossibleCallReachability. Defined separately
// here (not imported) for the same import-cycle reason documented on
// Diagnostic.
const KindPossibleCallReachability = "possible_call_reachability"

// ReachabilityStepFact is one node in a ReachabilityFactWire's Path,
// mirroring projectmodel.ReachabilityStep's NodeID field exactly.
type ReachabilityStepFact struct {
	NodeID string `json:"node_id"`
}

// ReachabilityFactWire is one possible-call-reachability observation
// reported by the sidecar, mirroring projectmodel.ReachabilityFact's field
// shape (ID/Kind/Confidence/Source/Sink/Path/AlgorithmVersion) with an added
// Backend field recording which analysis backend/language produced the
// fact.
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
// shape (phase, complete, counts, budgets, diagnostics) for the same
// import-cycle reason documented on Diagnostic. The client always sets its
// own Phase (the tsSidecarPhase constant), ignoring Response.Coverage.
// Phase entirely; on a non-Error Response it carries Complete, Counts,
// Budgets, and Diagnostics through unchanged, but on the Error path it
// substitutes client-synthesized Counts/Budgets instead of translating
// these fields.
type Coverage struct {
	Phase       string         `json:"phase"`
	Complete    bool           `json:"complete"`
	Counts      map[string]int `json:"counts,omitempty"`
	Budgets     map[string]int `json:"budgets,omitempty"`
	Diagnostics []Diagnostic   `json:"diagnostics,omitempty"`
}

// Response is one JSON response read from the sidecar's stdout, one line
// terminated by a single newline.
type Response struct {
	// Version echoes the Request's Version; the client rejects a Response
	// whose Version does not match its own ProtocolVersion instead of
	// misinterpreting a future incompatible wire shape.
	Version int `json:"version"`
	// ID echoes the Request's ID.
	ID          int64            `json:"id"`
	ImportEdges []ImportEdgeFact `json:"import_edges,omitempty"`
	// CallGraph and ReachabilityFacts are empty when the sidecar reports no
	// such facts; absence is never a "none exist" claim (see
	// projectmodel.ReachabilityFact).
	CallGraph         []CallGraphEdgeFact    `json:"call_graph,omitempty"`
	ReachabilityFacts []ReachabilityFactWire `json:"reachability_facts,omitempty"`
	Coverage          Coverage               `json:"coverage"`
	// Error is set for a whole-request failure (e.g. an unreadable
	// tsconfig); Coverage is still meaningful in that case -- the Go
	// client merges Coverage.Diagnostics into the diagnostics it returns,
	// alongside a synthesized DiagBackendUnavailable entry for Error
	// itself, though it always reports Complete false regardless of what
	// Coverage.Complete says.
	Error *ErrorPayload `json:"error,omitempty"`
}

// ErrorPayload is the wire form of a whole-request sidecar error, mirroring
// internal/jsbridge.ErrorPayload.
type ErrorPayload struct {
	// Kind is one of the Kind* constants. It never changes the diagnostic
	// code the Go client reports -- every Kind collapses to the stable
	// project_backend_unavailable diagnostic -- but the client appends it
	// to that diagnostic's message (e.g. "... (kind: crashed)") since Go's
	// errors.Is cannot cross the process boundary.
	Kind    string `json:"kind"`
	Message string `json:"message"`
}

// Error kinds for this boundary. Every kind here maps to the stable
// project_backend_unavailable diagnostic
// (pkg/projectmodel.DiagBackendUnavailable) on the Go client side, per
// issue #214's "Missing or failed sidecar behavior follows the stable
// backend-unavailable diagnostic" requirement: the Kind distinguishes the
// failure for logging/messages, but never changes the diagnostic code a
// caller of pkg/projectmodel observes.
const (
	// KindBackendUnavailable: the sidecar binary is missing, or the
	// process failed to start.
	KindBackendUnavailable = "backend_unavailable"
	// KindCrashed: the sidecar process exited non-zero.
	KindCrashed = "crashed"
	// KindInternal: the sidecar produced malformed or oversized output.
	KindInternal = "internal"
	// KindCanceled: the call was canceled or timed out before a response
	// was read.
	KindCanceled = "canceled"
)
