package projectmodel

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

const maxTSSidecarResponseBytes = 8 << 20 // 8 MiB

const maxTSSidecarStderrBytes = 4 << 10 // 4 KiB

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

func rootScopesFromWire(in []projectbridge.RootScopeFact) []RootScope {
	if len(in) == 0 {
		return nil
	}
	scopes := make([]RootScope, 0, len(in))
	for _, rs := range in {
		scopes = append(scopes, RootScope{
			Root:            rs.Root,
			CandidateFiles:  rs.CandidateFiles,
			AnalyzedFiles:   rs.AnalyzedFiles,
			AnalyzedPaths:   rs.AnalyzedPaths,
			UnanalyzedPaths: rs.UnanalyzedPaths,
		})
	}
	return scopes
}

// rootScopeIncompleteDiagnostics never reads reachability diagnostics --
// model completeness and reachability completeness are independent axes.
// The per-root fallback below covers a RootScope with counts but no path
// lists (a sidecar response predating UnanalyzedPaths).
func rootScopeIncompleteDiagnostics(scopes []RootScope) []Diagnostic {
	var diags []Diagnostic
	for _, scope := range scopes {
		if scope.AnalyzedFiles >= scope.CandidateFiles {
			continue
		}
		if len(scope.UnanalyzedPaths) > 0 {
			diags = append(diags, incompletePathDiagnostics(scope)...)
			continue
		}
		diags = append(diags, Diagnostic{
			Code:    DiagRootScopeIncomplete,
			Message: fmt.Sprintf("root %q: only %d of %d candidate files were incorporated into the import model", scope.Root, scope.AnalyzedFiles, scope.CandidateFiles),
			Path:    scope.Root,
		})
	}
	return diags
}

func incompletePathDiagnostics(scope RootScope) []Diagnostic {
	diags := make([]Diagnostic, 0, len(scope.UnanalyzedPaths))
	for _, path := range scope.UnanalyzedPaths {
		diags = append(diags, Diagnostic{
			Code:    DiagRootScopeIncomplete,
			Message: fmt.Sprintf("root %q: candidate file %q was never incorporated into the import model", scope.Root, path),
			Path:    path,
		})
	}
	return diags
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
