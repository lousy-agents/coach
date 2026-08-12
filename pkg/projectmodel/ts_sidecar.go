package projectmodel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
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

	edges := make([]ImportEdge, 0, len(resp.ImportEdges))
	for _, e := range resp.ImportEdges {
		edges = append(edges, ImportEdge{From: e.From, To: e.To, Kind: e.Kind, Site: e.Site, Resolution: e.Resolution})
	}
	diagnostics := make([]Diagnostic, 0, len(resp.Coverage.Diagnostics))
	for _, d := range resp.Coverage.Diagnostics {
		diagnostics = append(diagnostics, Diagnostic{Code: d.Code, Message: d.Message, Path: d.Path})
	}

	return Model{
		SchemaVersion: SchemaVersion,
		Repository:    meta.Repository,
		Snapshot:      tsSidecarSnapshot(meta, opts),
		ImportEdges:   edges,
		Coverage: canonicalCoverage(Coverage{
			Phase:       tsSidecarPhase,
			Complete:    resp.Coverage.Complete,
			Counts:      resp.Coverage.Counts,
			Budgets:     resp.Coverage.Budgets,
			Diagnostics: diagnostics,
		}),
	}, nil
}

// callTSSidecar runs the sidecar transport for one request: it stats the
// binary, spawns it with a minimal explicit environment and opts.Args,
// writes req to its stdin, and reads one bounded response line from its
// stdout. On any transport failure it returns a non-empty human-readable
// message describing which failure mode occurred (missing binary, failed
// start, non-zero exit, oversized output, or timeout); the caller turns
// that into a DiagBackendUnavailable diagnostic rather than a Go error.
func callTSSidecar(ctx context.Context, opts TSSidecarOptions, req projectbridge.Request) (projectbridge.Response, string) {
	if _, err := os.Stat(opts.BinaryPath); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar binary unavailable at %q: %s", opts.BinaryPath, err)
	}

	runCtx := ctx
	if opts.Timeout > 0 {
		var cancelTimeout context.CancelFunc
		runCtx, cancelTimeout = context.WithTimeout(runCtx, opts.Timeout)
		defer cancelTimeout()
	}
	runCtx, cancelRun := context.WithCancel(runCtx)
	defer cancelRun()

	reqLine, err := json.Marshal(req)
	if err != nil {
		return projectbridge.Response{}, fmt.Sprintf("encoding ts sidecar request: %s", err)
	}

	cmd := exec.CommandContext(runCtx, opts.BinaryPath, opts.Args...)
	cmd.Env = sanitizedTSSidecarEnv()
	cmd.Stdin = bytes.NewReader(append(reqLine, '\n'))
	stderr := &boundedWriter{limit: maxTSSidecarStderrBytes}
	cmd.Stderr = stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return projectbridge.Response{}, fmt.Sprintf("starting ts sidecar: %s", err)
	}
	if err := cmd.Start(); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("starting ts sidecar: %s", err)
	}

	data, readErr := io.ReadAll(io.LimitReader(stdout, maxTSSidecarResponseBytes+1))
	oversized := int64(len(data)) > maxTSSidecarResponseBytes
	// Only cancel before Wait when the child may still be writing past the
	// budget (oversized) or the read itself failed; canceling
	// unconditionally races exec.CommandContext's own ctx-watchdog against
	// a child that has already exited normally, and if the watchdog wins
	// it kills the (already-dead) process group and Wait returns
	// ctx.Err() instead of nil -- turning a perfectly good response into a
	// spurious "context canceled" failure. On the normal path stdout has
	// already hit EOF, so Wait returns on its own and the deferred
	// cancelRun still runs afterward.
	if oversized || readErr != nil {
		cancelRun()
	}
	waitErr := cmd.Wait()

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		if opts.Timeout > 0 {
			return projectbridge.Response{}, fmt.Sprintf("ts sidecar timed out after %s", opts.Timeout)
		}
		// opts.Timeout is documented as "no deadline" when zero, so the
		// DeadlineExceeded here came from a deadline the caller set on ctx
		// (e.g. context.WithDeadline/WithTimeout), not opts.Timeout --
		// printing opts.Timeout in that case would misreport it as 0s.
		return projectbridge.Response{}, "ts sidecar timed out (caller deadline exceeded)"
	}
	// ctx (the caller's original context, distinct from runCtx which this
	// function always cancels itself once the response is read) is only
	// Canceled here if the caller canceled it; that must be reported
	// distinctly from a crash, since cmd.Wait() otherwise reports the same
	// "exited" shape (e.g. "signal: killed") for both.
	if errors.Is(ctx.Err(), context.Canceled) {
		return projectbridge.Response{}, "ts sidecar canceled"
	}
	if oversized {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar response exceeded %d-byte budget", maxTSSidecarResponseBytes)
	}
	if readErr != nil {
		return projectbridge.Response{}, fmt.Sprintf("reading ts sidecar output: %s", readErr)
	}
	if waitErr != nil {
		if tail := strings.TrimSpace(stderr.buf.String()); tail != "" {
			return projectbridge.Response{}, fmt.Sprintf("ts sidecar exited: %s: %s", waitErr, tail)
		}
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar exited: %s", waitErr)
	}

	var resp projectbridge.Response
	if err := json.Unmarshal(bytes.TrimRight(firstLine(data), "\n"), &resp); err != nil {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar produced malformed response: %s", err)
	}
	if resp.Version != projectbridge.ProtocolVersion || resp.ID != req.ID {
		return projectbridge.Response{}, fmt.Sprintf("ts sidecar response protocol mismatch: version %d id %d", resp.Version, resp.ID)
	}
	return resp, ""
}

// sanitizedTSSidecarEnv is the minimal environment for the sidecar child,
// mirroring internal/codesignalcli/project_snapshot.go's
// sanitizedSnapshotGitEnv: only PATH and HOME are forwarded so the
// sidecar can locate itself and any runtime it wraps (e.g. a Node
// installation), never the parent process's full ambient environment.
func sanitizedTSSidecarEnv() []string {
	var env []string
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
	}
	return env
}

// boundedWriter retains only the first limit bytes written to it, silently
// discarding the rest, so an untrusted child's stderr cannot grow the
// diagnostic message without bound.
type boundedWriter struct {
	buf   bytes.Buffer
	limit int
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if room := w.limit - w.buf.Len(); room > 0 {
		if room > len(p) {
			room = len(p)
		}
		w.buf.Write(p[:room])
	}
	return len(p), nil
}

func firstLine(data []byte) []byte {
	if idx := bytes.IndexByte(data, '\n'); idx >= 0 {
		return data[:idx]
	}
	return data
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
	diags := make([]Diagnostic, 0, len(resp.Coverage.Diagnostics)+1)
	diags = append(diags, Diagnostic{Code: DiagBackendUnavailable, Message: message})
	for _, d := range resp.Coverage.Diagnostics {
		diags = append(diags, Diagnostic{Code: d.Code, Message: d.Message, Path: d.Path})
	}
	return diags
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

// collectTSSidecarFiles walks snapshot for .ts/.tsx source files (scoped
// to roots when non-empty, else the whole snapshot), base64-encoding each
// one's content for internal/projectbridge.Request.Files. It returns the
// files collected plus a separate seen count so a read failure still
// contributes to Coverage.Counts["files_seen"].
func collectTSSidecarFiles(snapshot fs.FS, roots []string) ([]projectbridge.ProjectFile, int) {
	var paths []string
	for _, root := range tsSidecarWalkRoots(roots) {
		_ = fs.WalkDir(snapshot, root, func(p string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return nil
			}
			if strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".tsx") {
				paths = append(paths, p)
			}
			return nil
		})
	}
	sort.Strings(paths)
	paths = dedupeSorted(paths)

	files := make([]projectbridge.ProjectFile, 0, len(paths))
	for _, p := range paths {
		content, err := fs.ReadFile(snapshot, p)
		if err != nil {
			continue
		}
		files = append(files, projectbridge.ProjectFile{
			Path:       p,
			ContentB64: base64.StdEncoding.EncodeToString(content),
		})
	}
	return files, len(paths)
}

func tsSidecarWalkRoots(roots []string) []string {
	if len(roots) == 0 {
		return []string{"."}
	}
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = path.Clean(r)
	}
	return out
}

func dedupeSorted(sortedPaths []string) []string {
	if len(sortedPaths) < 2 {
		return sortedPaths
	}
	out := sortedPaths[:1]
	for _, p := range sortedPaths[1:] {
		if p != out[len(out)-1] {
			out = append(out, p)
		}
	}
	return out
}
