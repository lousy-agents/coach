package codesignalcli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// goProjectBuildWallTime bounds one BuildGoModel call's reserved WallTime
// budget field. Matches epic #208's frozen "Snapshot/import facts" phase
// row in the "Initial resource defaults" table (60s), even though
// BuildGoModel does not enforce WallTime itself yet.
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

// goProjectBudgets bounds every BuildGoModel call goProjectBackend issues,
// pinned to epic #208's frozen "Snapshot/import facts" phase row in the
// "Initial resource defaults" table -- that issue states corpus
// measurements "may revise them only through a versioned contract update",
// so these values must track that table, not be chosen independently.
// Repository-controlled input must never be read unboundedly (see
// project_snapshot.go's maxSnapshotListBytes/maxSnapshotFileBytes and
// project.go's git-read budgets for the same convention). Only
// MaxInputFiles and MaxInputBytes are enforced by BuildGoModel today;
// WallTime/MaxGraphNodes/MaxGraphEdges/MaxWorkingSetBytes are reserved
// fields with no truncating effect yet, but are still set to the table's
// finite values here so Model.Coverage.Budgets never advertises
// "unbounded" for a dimension this backend intends to bound once
// enforcement lands.
var goProjectBudgets = projectmodel.GoBudgets{
	WallTime:           goProjectBuildWallTime,
	MaxInputFiles:      5000,
	MaxInputBytes:      goProjectMaxInputBytes,
	MaxGraphNodes:      100000,
	MaxGraphEdges:      500000,
	MaxWorkingSetBytes: 512 << 20,
}

// goProjectBackend is the ProjectBackend for --project-language go: it
// builds a projectmodel.Model from an immutable Git snapshot and evaluates
// it against the config's layer policy via
// codesignal.EvaluateGoLayerViolations.
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

func (b *goProjectBackend) Analyze(_ context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	var config projectConfig
	if err := json.Unmarshal(req.Config, &config); err != nil {
		return nil, fmt.Errorf("coach: decoding validated project config: %w", err)
	}
	policy := layerPolicyFromConfig(config)

	headChanges, headCoverage, err := b.evaluateRevision(req.Dir, req.HeadRevision, config.Roots, policy, req.ConfigDigest)
	if err != nil {
		return nil, err
	}

	result := &ProjectBackendResult{
		HeadChanges:  headChanges,
		HeadCoverage: &headCoverage,
	}
	if req.Baseline {
		return result, nil
	}

	baseChanges, baseCoverage, err := b.evaluateRevision(req.Dir, req.BaseRevision, config.Roots, policy, req.ConfigDigest)
	if err != nil {
		return nil, err
	}
	result.BaseChanges = baseChanges
	result.BaseCoverage = &baseCoverage
	result.BaseAnalyzed = true
	return result, nil
}

// evaluateRevision builds a Go project model at revision and evaluates it
// against policy, returning one architecture.layer_violation ProjectChange
// per violating package pair plus the model's own Coverage.
func (b *goProjectBackend) evaluateRevision(dir, revision string, roots []string, policy codesignal.LayerPolicy, configDigest string) ([]codesignal.ProjectChange, projectmodel.Coverage, error) {
	snapshot, err := NewGoSnapshotFS(dir, revision)
	if err != nil {
		return nil, projectmodel.Coverage{}, fmt.Errorf("coach: building Go snapshot at revision %q: %w", revision, err)
	}

	model, err := projectmodel.BuildGoModel(snapshot, projectmodel.SnapshotMeta{
		Revision:     revision,
		ConfigDigest: configDigest,
	}, projectmodel.GoBuildOptions{
		Roots:   roots,
		Budgets: goProjectBudgets,
	})
	if err != nil {
		return nil, projectmodel.Coverage{}, fmt.Errorf("coach: building Go project model at revision %q: %w", revision, err)
	}

	changes, _ := codesignal.EvaluateGoLayerViolations(model, policy, goLayerRuleVersion, goLayerBackendVersion, configDigest)
	return changes, model.Coverage, nil
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
