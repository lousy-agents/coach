package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/lousy-agents/coach/internal/projectbridge"
	"github.com/lousy-agents/coach/pkg/projectmodel/internal/tssidecar"
)

const tsSidecarPhase = "ts_sidecar_build"

type TSSidecarOptions struct {
	BinaryPath string
	Path       string
	Dir        string
	Roots      []string
	Args       []string
	Timeout    time.Duration
	Budgets    GoBudgets
}

// tsProjectAnalyzer is the port a TypeScript Model build sends its single
// analyze_project request through. tssidecar.Process is the production
// adapter; a returned error is a transport failure whose message becomes a
// DiagBackendUnavailable diagnostic.
type tsProjectAnalyzer interface {
	Analyze(ctx context.Context, req projectbridge.Request) (projectbridge.Response, error)
}

func BuildTypeScriptModelViaSidecar(ctx context.Context, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions) (Model, error) {
	analyzer := tssidecar.Process{
		BinaryPath: opts.BinaryPath,
		Path:       opts.Path,
		Dir:        opts.Dir,
		Args:       opts.Args,
		Timeout:    opts.Timeout,
	}
	return buildTypeScriptModel(ctx, analyzer, snapshot, meta, opts)
}

func buildTypeScriptModel(ctx context.Context, analyzer tsProjectAnalyzer, snapshot fs.FS, meta SnapshotMeta, opts TSSidecarOptions) (Model, error) {
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

	resp, transportErr := analyzer.Analyze(ctx, req)
	var model Model
	switch {
	case transportErr != nil:
		model = tsSidecarModel(meta, opts, filesSeen, Diagnostic{Code: DiagBackendUnavailable, Message: transportErr.Error()})
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
