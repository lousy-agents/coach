package codesignalcli

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"time"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// goProjectBuildWallTime bounds one BuildGoModel/BuildGoLayerBypass call's
// WallTime budget field. Matches epic #208's frozen "Snapshot/import facts"
// phase row in the "Initial resource defaults" table (60s). BuildGoModel
// does not enforce WallTime itself; BuildGoLayerBypass does, as a hard
// context.WithTimeout wall-clock deadline over its witness search (see
// pkg/projectmodel/go_layer_bypass.go's BuildGoLayerBypass).
const goProjectBuildWallTime = 60 * time.Second

// goProjectMaxInputBytes bounds the cumulative analyzed source bytes
// BuildGoModel reads across all files. It is a dedicated constant, not a
// reuse of maxSnapshotListBytes (project_snapshot.go's `git ls-tree -r`
// path-listing cap), because the two bound different dimensions: changing
// the ls-tree cap must not silently change this model-build budget. Set to
// 50 MiB per revision per epic #208's frozen "Snapshot/import facts" row;
// callers must not change this value without a versioned contract update
// (see that issue's "Initial resource defaults" table).
const goProjectMaxInputBytes = 50 << 20

// Identity constants for architecture.layer_violation ProjectChanges emitted
// by goProjectBackend. goLayerRuleVersion tracks EvaluateGoLayerViolations'
// own evaluation semantics (pkg/codesignal); goLayerBackendVersion tracks
// this backend's own build/wiring, independent of the evaluator. Both are
// part of a ProjectChange's lifecycle identity (see project_fingerprint.go)
// and must only change when the corresponding logic changes.
const (
	goLayerRuleVersion    = "1"
	goLayerBackendVersion = "go-layer-policy@1"
)

// Identity constants for architecture.layer_bypass ProjectChanges emitted by
// goProjectBackend. goBypassRuleVersion tracks EvaluateGoLayerBypass's own
// evaluation semantics (pkg/codesignal); goBypassBackendVersion tracks this
// backend's own build/wiring, independent of the evaluator -- mirroring the
// goLayerRuleVersion/goLayerBackendVersion split above.
const (
	goBypassRuleVersion    = "1"
	goBypassBackendVersion = "go-layer-bypass@1"
)

// goProjectBudgets bounds every BuildGoModel call goProjectBackend issues,
// and is also passed as-is to every BuildGoLayerBypass call when the config
// declares required_layer. It is pinned to epic #208's frozen "Snapshot/
// import facts" phase row in the "Initial resource defaults" table -- that
// issue states corpus measurements "may revise them only through a
// versioned contract update", so these values must track that table, not be
// chosen independently. Repository-controlled input must never be read
// unboundedly (see project_snapshot.go's
// maxSnapshotListBytes/maxSnapshotFileBytes and project.go's git-read
// budgets for the same convention). MaxInputFiles and MaxInputBytes are
// enforced by discoverGoProject (the snapshot walk both BuildGoModel and
// BuildGoLayerBypass perform before doing anything else); WallTime is
// additionally enforced by BuildGoLayerBypass only (see
// goProjectBuildWallTime); MaxGraphNodes and MaxGraphEdges are enforced by
// BuildGoLayerBypass's own call-graph construction (buildGoCallGraphFromLoaded)
// but not by BuildGoModel, which never builds a call graph. MaxWorkingSetBytes
// remains a reserved field with no truncating effect on either call yet, but
// is still set to the table's finite value here so Model.Coverage.Budgets
// never advertises "unbounded" for a dimension this backend intends to
// bound once enforcement lands. See goLayerBypassMaxSearchNodes for why
// LayerBypassOptions.MaxSearchNodes deliberately does not reuse
// MaxGraphNodes.
var goProjectBudgets = projectmodel.GoBudgets{
	WallTime:           goProjectBuildWallTime,
	MaxInputFiles:      5000,
	MaxInputBytes:      goProjectMaxInputBytes,
	MaxGraphNodes:      100000,
	MaxGraphEdges:      500000,
	MaxWorkingSetBytes: 512 << 20,
}

// buildGoLayerBypass is the BuildGoLayerBypass seam evaluateRevision calls.
// Tests may replace it to inject a deterministic LayerBypassResult (e.g. an
// incomplete Coverage) directly, mirroring project.go's
// gitCommandContext/runProjectConfigGit seam convention in this package,
// instead of forcing the real search to truncate by giving
// goLayerBypassMaxSearchNodes a finite value.
var buildGoLayerBypass func(ctx context.Context, snapshot fs.FS, opts projectmodel.LayerBypassOptions) (projectmodel.LayerBypassResult, error) = projectmodel.BuildGoLayerBypass

// goLayerBypassMaxSearchNodes bounds the total call-graph nodes
// BuildGoLayerBypass's witness search visits across every source's BFS
// traversal *combined* (LayerBypassOptions.MaxSearchNodes shares one
// nodesVisited counter across all sources -- see
// pkg/projectmodel/go_layer_bypass.go's searchLayerBypassWitnesses), not
// per handler. This is deliberately 0 (unbounded): do not give it a finite
// default, and in particular do not reuse goProjectBudgets.MaxGraphNodes
// here -- that ceiling bounds one call graph's total size, not a search
// that revisits nodes once per source, so a repo with only a few dozen
// handler-shaped sources sharing one call graph would exhaust it and
// degrade every architecture.layer_bypass and architecture.layer_violation
// ProjectChange in the report to lifecycle "unknown" even though nothing
// pathological happened. The real production bound on this search's cost
// is goProjectBuildWallTime's hard context deadline (BuildGoLayerBypass
// checks ctx.Err() per BFS node/source), combined with the call graph's own
// finite size from MaxGraphNodes/MaxGraphEdges (enforced by
// buildGoCallGraphFromLoaded) -- so a single traversal can never exceed the
// graph's own capped size regardless of this value.
const goLayerBypassMaxSearchNodes = 0

// goProjectBackend is the ProjectBackend for --project-language go: it
// builds a projectmodel.Model from an immutable Git snapshot and evaluates
// it against the config's layer policy via
// codesignal.EvaluateGoLayerViolations. When the config declares
// required_layer, it additionally runs projectmodel.BuildGoLayerBypass over
// the same snapshot and evaluates the result via
// codesignal.EvaluateGoLayerBypass, appending any architecture.layer_bypass
// ProjectChanges alongside the layer-violation ones (issue #253).
type goProjectBackend struct{}

// NewGoProjectBackend returns the ProjectBackend registered for
// --project-language go. Analyze always builds the head-side model; when
// req.Baseline is false (diff mode) it additionally builds the base-side
// model and sets ProjectBackendResult.BaseAnalyzed. A revision that cannot
// be read as a Git snapshot (see NewGoSnapshotFS) is returned as a real
// error, not swallowed into an empty result.
func NewGoProjectBackend() ProjectBackend {
	return &goProjectBackend{}
}

func (b *goProjectBackend) Analyze(ctx context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	var config projectConfig
	if err := json.Unmarshal(req.Config, &config); err != nil {
		return nil, fmt.Errorf("coach: decoding validated project config: %w", err)
	}
	policy := layerPolicyFromConfig(config)
	bypassLayer, hasBypassLayer := goBypassLayerFromConfig(config)

	headChanges, headDiagnostics, headCoverage, err := b.evaluateRevision(ctx, req.Dir, req.HeadRevision, config.Roots, policy, bypassLayer, hasBypassLayer, req.ConfigDigest)
	if err != nil {
		return nil, err
	}

	result := &ProjectBackendResult{
		HeadChanges:     headChanges,
		HeadDiagnostics: headDiagnostics,
		HeadCoverage:    &headCoverage,
	}
	if req.Baseline {
		return result, nil
	}

	baseChanges, baseDiagnostics, baseCoverage, err := b.evaluateRevision(ctx, req.Dir, req.BaseRevision, config.Roots, policy, bypassLayer, hasBypassLayer, req.ConfigDigest)
	if err != nil {
		return nil, err
	}
	result.BaseChanges = baseChanges
	result.BaseDiagnostics = baseDiagnostics
	result.BaseCoverage = &baseCoverage
	result.BaseAnalyzed = true
	return result, nil
}

// evaluateRevision builds a Go project model at revision and evaluates it
// against policy, returning one architecture.layer_violation ProjectChange
// per violating package pair plus the model's own Coverage. When
// hasBypassLayer is true, it additionally runs BuildGoLayerBypass over the
// same snapshot and appends one architecture.layer_bypass ProjectChange per
// high-confidence witness EvaluateGoLayerBypass returns.
//
// Diagnostics returned alongside EvaluateGoLayerViolations' ProjectChanges
// are discarded, matching that evaluator's existing convention here.
// EvaluateGoLayerBypass's diagnostics are not discarded: its
// project_layer_bypass_coverage_incomplete diagnostic must reach the
// report, so it is returned alongside the ProjectChanges.
// LayerBypassResult.Coverage is folded into the returned Coverage via
// combineProjectCoverage so an incomplete bypass search degrades the
// project's overall Coverage.Complete -- and therefore
// projectLifecycleState's lifecycle-unknown gate (pkg/codesignal/codesignal.go)
// -- exactly like any other incomplete project-model phase, rather than
// silently reporting a "resolved"/"introduced"/"existing" claim on a witness
// that only disappeared because the search was truncated.
func (b *goProjectBackend) evaluateRevision(ctx context.Context, dir, revision string, roots []string, policy codesignal.LayerPolicy, bypassLayer projectmodel.BypassLayer, hasBypassLayer bool, configDigest string) ([]codesignal.ProjectChange, []codesignal.Diagnostic, projectmodel.Coverage, error) {
	snapshot, err := NewGoSnapshotFS(dir, revision)
	if err != nil {
		return nil, nil, projectmodel.Coverage{}, fmt.Errorf("coach: building Go snapshot at revision %q: %w", revision, err)
	}

	model, err := projectmodel.BuildGoModel(snapshot, projectmodel.SnapshotMeta{
		Revision:     revision,
		ConfigDigest: configDigest,
	}, projectmodel.GoBuildOptions{
		Roots:   roots,
		Budgets: goProjectBudgets,
	})
	if err != nil {
		return nil, nil, projectmodel.Coverage{}, fmt.Errorf("coach: building Go project model at revision %q: %w", revision, err)
	}

	changes, _ := codesignal.EvaluateGoLayerViolations(model, policy, goLayerRuleVersion, goLayerBackendVersion, configDigest)
	coverage := model.Coverage
	var diagnostics []codesignal.Diagnostic

	if hasBypassLayer {
		bypassResult, err := buildGoLayerBypass(ctx, snapshot, projectmodel.LayerBypassOptions{
			RequiredLayer:  bypassLayer,
			Roots:          roots,
			Budgets:        goProjectBudgets,
			MaxSearchNodes: goLayerBypassMaxSearchNodes,
		})
		if err != nil {
			return nil, nil, projectmodel.Coverage{}, fmt.Errorf("coach: building Go layer-bypass model at revision %q: %w", revision, err)
		}
		bypassChanges, bypassDiagnostics := codesignal.EvaluateGoLayerBypass(bypassResult, goBypassRuleVersion, goBypassBackendVersion, configDigest)
		changes = append(changes, bypassChanges...)
		diagnostics = append(diagnostics, bypassDiagnostics...)
		coverage = combineProjectCoverage(coverage, bypassResult.Coverage)
	}

	return changes, diagnostics, coverage, nil
}

// combineProjectCoverage folds bypass (a LayerBypassResult's own Coverage)
// into model (the Go project model's own Coverage): Complete is ANDed, since
// either phase not completing means the combined result cannot claim a
// complete project analysis, and Diagnostics is unioned so bypass's own
// projectmodel-native diagnostics (e.g. a budget-exceeded or unresolved-root
// code) stay visible on the reported Coverage rather than being dropped when
// only one of the two phases actually failed. Phase/Counts/Budgets are kept
// from model: the two phases measure different dimensions (project-model
// build vs. one layer-bypass search), so merging their counts/budgets would
// conflate incomparable numbers rather than clarify anything.
func combineProjectCoverage(model, bypass projectmodel.Coverage) projectmodel.Coverage {
	combined := model
	combined.Complete = model.Complete && bypass.Complete
	if len(bypass.Diagnostics) > 0 {
		combined.Diagnostics = append(append([]projectmodel.Diagnostic(nil), model.Diagnostics...), bypass.Diagnostics...)
	}
	return combined
}

// goBypassLayerFromConfig resolves config.RequiredLayer -- already validated
// by LoadProjectConfig to either be empty or name a declared layer -- into
// the projectmodel.BypassLayer BuildGoLayerBypass expects. The second return
// value is false when RequiredLayer is unset, in which case goProjectBackend
// skips the layer-bypass search entirely: BuildGoLayerBypass would otherwise
// treat a zero-value BypassLayer as ambiguous (see
// projectmodel.DiagLayerBypassAmbiguousLayer) and report incomplete coverage
// for a search the config never asked for.
func goBypassLayerFromConfig(config projectConfig) (projectmodel.BypassLayer, bool) {
	if config.RequiredLayer == "" {
		return projectmodel.BypassLayer{}, false
	}
	for _, layer := range config.Layers {
		if layer.Name == config.RequiredLayer {
			return projectmodel.BypassLayer{Name: layer.Name, Prefixes: append([]string(nil), layer.Prefixes...)}, true
		}
	}
	return projectmodel.BypassLayer{}, false
}

// layerPolicyFromConfig translates the already-schema-validated
// projectConfig layers/forbidden_imports into codesignal.LayerPolicy. It is
// language-agnostic (projectConfig/LayerPolicy have no Go- or TS-specific
// fields), so both goProjectBackend and tsProjectBackend share it.
func layerPolicyFromConfig(config projectConfig) codesignal.LayerPolicy {
	layers := make([]codesignal.ArchitectureLayer, len(config.Layers))
	for i, layer := range config.Layers {
		layers[i] = codesignal.ArchitectureLayer{
			Name:     layer.Name,
			Prefixes: append([]string(nil), layer.Prefixes...),
		}
	}
	forbidden := make([]codesignal.ForbiddenLayerImport, len(config.ForbiddenImports))
	for i, f := range config.ForbiddenImports {
		forbidden[i] = codesignal.ForbiddenLayerImport{From: f.From, To: f.To}
	}
	return codesignal.LayerPolicy{Layers: layers, ForbiddenImports: forbidden}
}
