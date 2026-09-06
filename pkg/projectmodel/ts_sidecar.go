package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// DiagBackendUnavailable is the stable diagnostic code for every TypeScript
// sidecar transport failure: missing binary, failed start, non-zero exit,
// malformed/oversized output, or timeout/cancellation. It reuses the exact
// "project_backend_unavailable" string embedded in
// internal/codesignalcli.ProjectBackendUnavailableError's message so the
// CLI layer and this package's own sidecar diagnostics agree on one
// identifier, per issue #214's requirement that missing or failed sidecar
// behavior follow one stable backend-unavailable diagnostic.
const DiagBackendUnavailable = "project_backend_unavailable"

// maxTSSidecarResponseBytes bounds the single NDJSON response line read
// from the sidecar's stdout, mirroring the bounded-read spirit of
// runGitBytesBoundedWith in internal/codesignalcli/project.go: an
// oversized or hung sidecar must fail closed rather than let the client
// buffer unbounded memory.
const maxTSSidecarResponseBytes = 8 << 20 // 8 MiB

// maxTSSidecarStderrBytes bounds how much of the sidecar child's stderr is
// retained for inclusion in a crash diagnostic. A misbehaving child's
// stderr is untrusted, unbounded input, so it is capped the same way
// maxTSSidecarResponseBytes caps stdout.
const maxTSSidecarStderrBytes = 4 << 10 // 4 KiB

// tsSidecarPhase is Model.Coverage.Phase for every BuildTypeScriptModelViaSidecar
// call, mirroring BuildGoModel's "go_model_build" convention.
const tsSidecarPhase = "ts_sidecar_build"

// TSSidecarOptions bounds one BuildTypeScriptModelViaSidecar call.
type TSSidecarOptions struct {
	// BinaryPath is the executable spawned for the sidecar, not a $PATH
	// lookup. Production TypeScript analysis passes the resolved Node
	// binary; tests pass a fake sidecar binary.
	BinaryPath string
	// Dir is the child's working directory. Empty inherits the parent
	// process cwd. Production TypeScript analysis sets this to the
	// materialized private analyzer directory.
	Dir string
	// Roots optionally scopes file collection and sidecar analysis to
	// specific repository-relative project roots; empty scans the whole
	// snapshot.
	Roots []string
	// Args are extra command-line arguments passed to the sidecar binary
	// verbatim. The request itself always travels over stdin, not argv;
	// this exists for flags the sidecar binary itself needs (e.g. a
	// future --project flag), not for protocol data.
	Args []string
	// Timeout bounds both the client-side context deadline for the
	// sidecar subprocess and the TimeoutMS the sidecar is asked to
	// self-enforce (internal/projectbridge.Request.TimeoutMS), mirroring
	// internal/jsbridge's context.WithTimeout convention. Zero means no
	// deadline; since an oversized-output read can otherwise block
	// indefinitely on a stalled child, callers should always set a
	// positive Timeout in production.
	Timeout time.Duration
	// Budgets bounds collectTSSidecarFiles's input walk before any file is
	// sent to the sidecar, mirroring GoBuildOptions.Budgets -- only
	// MaxInputFiles and MaxInputBytes are enforced here (the same two
	// dimensions BuildGoModel enforces); the rest of GoBudgets' fields are
	// reserved for this backend the same way they are for Go's. A zero
	// value is unbounded, matching GoBudgets' own no-implicit-default
	// convention.
	Budgets GoBudgets
}

// BuildTypeScriptModelViaSidecar builds a Model's raw TypeScript/TSX
// import facts by spawning opts.BinaryPath as a subprocess, sending it one
// internal/projectbridge.Request over stdin, and reading one Response back
// from stdout. It performs no TypeScript analysis itself -- resolving
// tsconfig project references, path aliases, and package.json exports/
// re-exports is the sidecar's job (issue #214 Task 2); this function is
// pure transport plus response translation.
//
// Mirroring BuildGoModel's contract, BuildTypeScriptModelViaSidecar never
// returns a non-nil error for an unavailable, crashed, malformed, or
// timed-out sidecar -- those are operational conditions, not programming
// errors, and are reported as a DiagBackendUnavailable entry in the
// returned Model's Coverage.Diagnostics (Coverage.Complete false) instead.
// A non-nil error is reserved for genuine programming-error conditions,
// such as a nil snapshot.
func BuildTypeScriptModelViaSidecar(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions) (Model, error) {
	if snapshot == nil {
		return Model{}, fmt.Errorf("projectmodel: snapshot must not be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	files, filesSeen, truncated := collectTSSidecarFiles(snapshot, opts.Roots, opts.Budgets)
	req := projectbridge.Request{
		Version:   projectbridge.ProtocolVersion,
		Op:        projectbridge.OpAnalyzeProject,
		ID:        1,
		Files:     files,
		Roots:     selectedRootsFrom(opts.Roots),
		TimeoutMS: opts.Timeout.Milliseconds(),
	}

	resp, diagMessage := callTSSidecar(ctx, opts, req)
	var model Model
	switch {
	case diagMessage != "":
		model = tsSidecarModel(meta, opts, filesSeen, Diagnostic{Code: DiagBackendUnavailable, Message: diagMessage})
	case resp.Error != nil:
		model = tsSidecarModel(meta, opts, filesSeen, tsSidecarErrorDiagnostics(resp)...)
	default:
		model = modelFromTSSidecarResponse(meta, opts, resp, files)
	}
	if truncated {
		model = applyTSSidecarInputBudgetTruncation(model)
	}
	return model, nil
}

// applyTSSidecarInputBudgetTruncation marks model incomplete and appends a
// DiagFileBudgetExceeded diagnostic when opts.Budgets truncated
// collectTSSidecarFiles's input walk, regardless of which outcome branch
// (successful response, resp.Error, or transport failure) produced model --
// a truncated input snapshot never represents complete facts, even if the
// sidecar itself reported success over the smaller set it was actually
// given.
func applyTSSidecarInputBudgetTruncation(model Model) Model {
	model.Coverage = canonicalCoverage(Coverage{
		Phase:    model.Coverage.Phase,
		Complete: false,
		Counts:   model.Coverage.Counts,
		Budgets:  model.Coverage.Budgets,
		Diagnostics: append(append([]Diagnostic{}, model.Coverage.Diagnostics...), Diagnostic{
			Code:    DiagFileBudgetExceeded,
			Message: "ts sidecar input collection truncated by Budgets.MaxInputFiles/MaxInputBytes",
		}),
	})
	return model
}

func modelFromTSSidecarResponse(meta SnapshotMeta, opts TSSidecarOptions, resp projectbridge.Response, files []projectbridge.ProjectFile) Model {
	return Model{
		SchemaVersion:     SchemaVersion,
		Repository:        meta.Repository,
		Snapshot:          tsSidecarSnapshot(meta, opts),
		Workspaces:        tsWorkspaceFactsFromCollected(files),
		Files:             tsFileFactsFromCollected(files),
		ImportEdges:       importEdgesFromWire(resp.ImportEdges),
		CallFacts:         callFactsFromWire(resp.CallGraph),
		ReachabilityFacts: reachabilityFactsFromWire(resp.ReachabilityFacts),
		Coverage: canonicalCoverage(Coverage{
			Phase:       tsSidecarPhase,
			Complete:    resp.Coverage.Complete,
			Counts:      resp.Coverage.Counts,
			Budgets:     resp.Coverage.Budgets,
			Diagnostics: diagnosticsFromWire(resp.Coverage.Diagnostics),
		}),
	}
}

func importEdgesFromWire(in []projectbridge.ImportEdgeFact) []ImportEdge {
	edges := make([]ImportEdge, 0, len(in))
	for _, e := range in {
		edges = append(edges, ImportEdge{From: e.From, To: e.To, Kind: e.Kind, Site: e.Site, Resolution: e.Resolution})
	}
	return edges
}

// callFactsFromWire translates the sidecar's raw call-graph edges into
// Model.CallFacts. Every non-error Response has attempted call-graph
// collection (analyze_project always tries it alongside import-edge
// extraction), so this always returns a non-nil slice -- an empty result set
// is "selected but found nothing", matching CallFacts' documented
// nil-vs-empty-slice contract, never "not selected".
func callFactsFromWire(in []projectbridge.CallGraphEdgeFact) []CallFact {
	facts := make([]CallFact, 0, len(in))
	for _, f := range in {
		facts = append(facts, CallFact{From: f.From, To: f.To})
	}
	return facts
}

// reachabilityFactsFromWire translates the sidecar's raw
// possible-call-reachability facts into Model.ReachabilityFacts, mirroring
// callFactsFromWire's always-non-nil contract. It deliberately drops the
// wire's Backend field: every Model BuildTypeScriptModelViaSidecar returns
// is TS-sourced by construction, so that provenance is already visible at
// the Model level (Coverage.Phase, Workspace/File.Language) without needing
// a per-fact field on projectmodel.ReachabilityFact, which has no Backend
// field today (see go_reachability.go). This mirrors the established
// wire-vs-model asymmetry documented on
// wire_bridge_parity_test.go's TestReachabilityBypassWireFieldParity: every
// projectbridge wire type in this family adds a trailing Backend provenance
// field with no projectmodel counterpart.
func reachabilityFactsFromWire(in []projectbridge.ReachabilityFactWire) []ReachabilityFact {
	facts := make([]ReachabilityFact, 0, len(in))
	for _, f := range in {
		facts = append(facts, ReachabilityFact{
			ID:               f.ID,
			Kind:             f.Kind,
			Confidence:       ReachabilityConfidence(f.Confidence),
			Source:           f.Source,
			Sink:             f.Sink,
			Path:             reachabilityStepsFromWire(f.Path),
			AlgorithmVersion: f.AlgorithmVersion,
		})
	}
	return facts
}

func reachabilityStepsFromWire(in []projectbridge.ReachabilityStepFact) []ReachabilityStep {
	steps := make([]ReachabilityStep, 0, len(in))
	for _, s := range in {
		steps = append(steps, ReachabilityStep{NodeID: s.NodeID})
	}
	return steps
}

func diagnosticsFromWire(in []projectbridge.Diagnostic) []Diagnostic {
	out := make([]Diagnostic, 0, len(in))
	for _, d := range in {
		out = append(out, Diagnostic{Code: d.Code, Message: d.Message, Path: d.Path})
	}
	return out
}

// tsSidecarModel returns the mostly-empty Model produced for a transport
// failure or a resp.Error whole-request failure: Coverage.Complete false
// with diags, per BuildTypeScriptModelViaSidecar's
// fail-open-with-diagnostics contract.
func tsSidecarModel(meta SnapshotMeta, opts TSSidecarOptions, filesSeen int, diags ...Diagnostic) Model {
	return Model{
		SchemaVersion: SchemaVersion,
		Repository:    meta.Repository,
		Snapshot:      tsSidecarSnapshot(meta, opts),
		Coverage: canonicalCoverage(Coverage{
			Phase:       tsSidecarPhase,
			Complete:    false,
			Counts:      map[string]int{"files_seen": filesSeen},
			Budgets:     map[string]int{"wall_time_ms": int(opts.Timeout / time.Millisecond)},
			Diagnostics: diags,
		}),
	}
}

// tsSidecarErrorDiagnostics translates a resp.Error whole-request failure
// into diagnostics: one DiagBackendUnavailable entry carrying the error's
// message plus its Kind (Kind never changes the diagnostic code -- every
// Kind collapses to DiagBackendUnavailable per
// internal/projectbridge.ErrorPayload's contract -- so it is folded into
// the message instead), followed by whatever partial resp.Coverage.
// Diagnostics the sidecar collected before failing.
func tsSidecarErrorDiagnostics(resp projectbridge.Response) []Diagnostic {
	message := resp.Error.Message
	if resp.Error.Kind != "" {
		message = fmt.Sprintf("%s (kind: %s)", message, resp.Error.Kind)
	}
	diags := []Diagnostic{{Code: DiagBackendUnavailable, Message: message}}
	return append(diags, diagnosticsFromWire(resp.Coverage.Diagnostics)...)
}

func tsSidecarSnapshot(meta SnapshotMeta, opts TSSidecarOptions) Snapshot {
	return Snapshot{
		Revision:           meta.Revision,
		TreeID:             meta.TreeID,
		ConfigDigest:       meta.ConfigDigest,
		BackendDigest:      meta.BackendDigest,
		BuildContextDigest: meta.BuildContextDigest,
		SelectedRoots:      selectedRootsFrom(opts.Roots),
	}
}
