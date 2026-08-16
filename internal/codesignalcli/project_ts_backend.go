package codesignalcli

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// tsSidecarWallTime bounds one BuildTypeScriptModelViaSidecar call's
// TSSidecarOptions.Timeout, matching goProjectBuildWallTime's 60s value from
// epic #208's frozen "Snapshot/import facts" phase row -- the TS sidecar
// call is a peer of the Go in-process build for budget purposes even though
// it crosses a subprocess boundary.
const tsSidecarWallTime = 60 * time.Second

// tsProjectMaxInputBytes mirrors goProjectMaxInputBytes for the TS sidecar's
// input-collection budget (TSSidecarOptions.Budgets), per epic #208's same
// frozen resource-default table.
const tsProjectMaxInputBytes = 50 << 20

// Identity constants for architecture.layer_violation ProjectChanges emitted
// by tsProjectBackend, kept distinct from goLayerRuleVersion/
// goLayerBackendVersion (see project_go_backend.go) since RuleVersion/
// BackendVersion identify each language backend's own evaluation/build
// wiring independently.
const (
	tsLayerRuleVersion    = "1"
	tsLayerBackendVersion = "ts-layer-policy@1"
)

// tsProjectBudgets mirrors goProjectBudgets for the TS sidecar backend; see
// that value's doc comment for the shared epic #208 resource-default table
// this must track.
var tsProjectBudgets = projectmodel.GoBudgets{
	WallTime:           tsSidecarWallTime,
	MaxInputFiles:      5000,
	MaxInputBytes:      tsProjectMaxInputBytes,
	MaxGraphNodes:      100000,
	MaxGraphEdges:      500000,
	MaxWorkingSetBytes: 512 << 20,
}

// tsSidecarRelativePath is the fixed, repository-pinned path (relative to
// the repository root, resolved via repositoryRoot -- see scope.go) that
// js/semantics/scripts/build-project-sidecar.mjs writes the compiled
// sidecar binary to. It is resolved per-request rather than once at
// construction, since the repository root varies per invocation while
// NewTSProjectBackend takes no arguments.
//
// req.Dir is os.Getwd() at CLI invocation time, which need not be the
// repository's checkout root (e.g. coach codesignal run from a
// subdirectory) -- so the base directory is resolved via repositoryRoot,
// not req.Dir itself. --project-language typescript only finds a sidecar
// when the analyzed repository itself vendors
// js/semantics/bin/coach-ts-project-sidecar at this exact
// repository-root-relative path. Every other repository silently degrades
// to DiagBackendUnavailable with Coverage.Complete false (exit 0, not an
// error): this is a deliberate, narrow v1 scope boundary, not a bug -- do
// not "fix" it into a $PATH or install-relative lookup.
const tsSidecarRelativePath = "js/semantics/bin/coach-ts-project-sidecar"

// tsProjectBackend is the ProjectBackend for --project-language typescript:
// it builds a projectmodel.Model per revision via a pinned local sidecar
// subprocess (BuildTypeScriptModelViaSidecar) and evaluates it against the
// config's layer policy via codesignal.EvaluateTypeScriptLayerViolations.
type tsProjectBackend struct{}

// NewTSProjectBackend returns the ProjectBackend registered for
// --project-language typescript. Analyze always builds the head-side model;
// when req.Baseline is false (diff mode) it additionally builds the
// base-side model and sets ProjectBackendResult.BaseAnalyzed.
//
// Unlike goProjectBackend, a missing, crashed, or timed-out sidecar is never
// surfaced as a Go error here: BuildTypeScriptModelViaSidecar's own contract
// reports that condition as a DiagBackendUnavailable diagnostic inside the
// returned Model's Coverage (Coverage.Complete false), which flows through
// unchanged into ProjectBackendResult.HeadCoverage/BaseCoverage.
func NewTSProjectBackend() ProjectBackend {
	return &tsProjectBackend{}
}

func (b *tsProjectBackend) Analyze(ctx context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	var config projectConfig
	if err := json.Unmarshal(req.Config, &config); err != nil {
		return nil, fmt.Errorf("coach: decoding validated project config: %w", err)
	}
	policy := layerPolicyFromConfig(config)

	// req.Dir need not be the repository root (see tsSidecarRelativePath);
	// repositoryRoot failing here means req.Dir is not inside a Git work
	// tree at all, which is the same fatal condition evaluateRevision's own
	// NewGoSnapshotFS call already surfaces as a hard error below -- so this
	// follows that same established convention rather than degrading to a
	// DiagBackendUnavailable diagnostic.
	root, err := repositoryRoot(req.Dir)
	if err != nil {
		return nil, fmt.Errorf("coach: resolving repository root for TypeScript sidecar binary: %w", err)
	}
	binaryPath := filepath.Join(root, tsSidecarRelativePath)

	headChanges, headCoverage, err := b.evaluateRevision(ctx, req.Dir, req.HeadRevision, binaryPath, config.Roots, policy, req.ConfigDigest)
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

	baseChanges, baseCoverage, err := b.evaluateRevision(ctx, req.Dir, req.BaseRevision, binaryPath, config.Roots, policy, req.ConfigDigest)
	if err != nil {
		return nil, err
	}
	result.BaseChanges = baseChanges
	result.BaseCoverage = &baseCoverage
	result.BaseAnalyzed = true
	return result, nil
}

// evaluateRevision builds a TypeScript project model at revision and
// evaluates it against policy, returning one architecture.layer_violation
// ProjectChange per violating (importer file, importee file) pair plus the
// model's own Coverage. NewGoSnapshotFS is reused as-is despite its Go-
// specific name: it is a plain immutable Git-revision fs.FS with no
// Go-specific behavior, the same snapshot mechanism goProjectBackend uses.
func (b *tsProjectBackend) evaluateRevision(ctx context.Context, dir, revision, binaryPath string, roots []string, policy codesignal.LayerPolicy, configDigest string) ([]codesignal.ProjectChange, projectmodel.Coverage, error) {
	snapshot, err := NewGoSnapshotFS(dir, revision)
	if err != nil {
		return nil, projectmodel.Coverage{}, fmt.Errorf("coach: building TypeScript snapshot at revision %q: %w", revision, err)
	}

	model, err := projectmodel.BuildTypeScriptModelViaSidecar(ctx, snapshot, projectmodel.SnapshotMeta{
		Revision:     revision,
		ConfigDigest: configDigest,
	}, projectmodel.TSSidecarOptions{
		BinaryPath: binaryPath,
		Roots:      roots,
		Timeout:    tsSidecarWallTime,
		Budgets:    tsProjectBudgets,
	})
	if err != nil {
		return nil, projectmodel.Coverage{}, fmt.Errorf("coach: building TypeScript project model at revision %q: %w", revision, err)
	}

	changes, _ := codesignal.EvaluateTypeScriptLayerViolations(model, policy, tsLayerRuleVersion, tsLayerBackendVersion, configDigest)
	return changes, model.Coverage, nil
}
