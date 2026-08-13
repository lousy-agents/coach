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
	// BinaryPath is a repository-pinned path to the sidecar executable,
	// not a $PATH lookup -- mirroring js/semantics/src/backend-cli.ts's
	// fixed BINARY_URL built-artifact path rather than searching PATH.
	BinaryPath string
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

	files, filesSeen := collectTSSidecarFiles(snapshot, opts.Roots)
	req := projectbridge.Request{
		Version:   projectbridge.ProtocolVersion,
		Op:        projectbridge.OpAnalyzeProject,
		ID:        1,
		Files:     files,
		Roots:     selectedRootsFrom(opts.Roots),
		TimeoutMS: opts.Timeout.Milliseconds(),
	}

	resp, diagMessage := callTSSidecar(ctx, opts, req)
	if diagMessage != "" {
		return tsSidecarModel(meta, opts, filesSeen, Diagnostic{Code: DiagBackendUnavailable, Message: diagMessage}), nil
	}
	if resp.Error != nil {
		return tsSidecarModel(meta, opts, filesSeen, tsSidecarErrorDiagnostics(resp)...), nil
	}
	return modelFromTSSidecarResponse(meta, opts, resp), nil
}

func modelFromTSSidecarResponse(meta SnapshotMeta, opts TSSidecarOptions, resp projectbridge.Response) Model {
	return Model{
		SchemaVersion: SchemaVersion,
		Repository:    meta.Repository,
		Snapshot:      tsSidecarSnapshot(meta, opts),
		ImportEdges:   importEdgesFromWire(resp.ImportEdges),
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
